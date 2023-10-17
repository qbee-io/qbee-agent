// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/qbee-io/qbee-agent/app/remoteaccess"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// FirewallBundle configures system firewall.
//
// Example payload:
//
//	{
//	 "tables": {
//	   "filter": {
//	     "INPUT": {
//	       "policy": "ACCEPT",
//	       "rules": [
//	         {
//	           "srcIp": "192.168.1.1",
//	           "dstPort": "80",
//	           "proto": "tcp",
//	           "target": "ACCEPT"
//	         }
//	       ]
//	     }
//	   }
//	 }
//	}
type FirewallBundle struct {
	Metadata

	// Tables defines a map of firewall tables to be modified.
	Tables map[FirewallTableName]FirewallTable `json:"tables"`
}

// Execute firewall configuration bundle on the system.
func (f FirewallBundle) Execute(ctx context.Context, service *Service) error {
	for tableName, chains := range f.Tables {
		for chainName, chain := range chains {
			if err := chain.execute(ctx, tableName, chainName); err != nil {
				return err
			}
		}
	}

	return nil
}

// FirewallTableName defines which firewall table name.
type FirewallTableName string

// Supported firewall table names.
const (
	Filter FirewallTableName = "filter"
	NAT    FirewallTableName = "nat"
)

// FirewallChainName defines firewall table's chain name.
type FirewallChainName string

// Supported firewall chain names.
const (
	Input       FirewallChainName = "INPUT"
	Forward     FirewallChainName = "FORWARD"
	Output      FirewallChainName = "OUTPUT"
	PreRouting  FirewallChainName = "PREROUTING"
	PostRouting FirewallChainName = "POSTROUTING"
)

// Protocol defines network protocol in use.
type Protocol string

// Supported network protocols.
const (
	TCP  Protocol = "tcp"
	UDP  Protocol = "udp"
	ICMP Protocol = "icmp"
)

// Target defines what to do with matching packets.
type Target string

// Supported iptables targets.
const (
	Accept Target = "ACCEPT"
	Drop   Target = "DROP"
	Reject Target = "REJECT"
)

// FirewallTable defines chains configuration for a firewall table.
type FirewallTable map[FirewallChainName]FirewallChain

// FirewallChain contains rules definition for a firewall chain.
type FirewallChain struct {
	// Policy defines a default policy (if no rule can be matched).
	Policy Target `json:"policy"`

	// Rules defines a list of firewall rules for a chain.
	Rules []FirewallRule `json:"rules"`
}

// execute a firewall chain configuration.
func (c FirewallChain) execute(ctx context.Context, table FirewallTableName, chain FirewallChainName) error {
	iptablesBin, err := exec.LookPath("iptables")
	if err != nil {
		ReportError(ctx, err, "Firewall configuration failed.")
		return err
	}

	// list current rules for a table and chain
	listRulesCmd := []string{iptablesBin, "-t", string(table), "-S", string(chain)}
	var currentRules []byte
	if currentRules, err = utils.RunCommand(ctx, listRulesCmd); err != nil {
		ReportError(ctx, err, "Firewall configuration failed.")
		return err
	}

	// make expected rules-set
	expectedRules := c.Render(table, chain, false)

	// current state is correct, nothing to do
	if bytes.Equal(bytes.TrimSpace(currentRules), []byte(strings.Join(expectedRules, "\n"))) {
		return nil
	}

	ReportWarning(ctx, currentRules, "Current firewall rules are not in compliance.")

	// flush all rules
	flushCmd := []string{iptablesBin, "-t", string(table), "-F", string(chain)}
	if _, err = utils.RunCommand(ctx, flushCmd); err != nil {
		ReportError(ctx, err, "Firewall configuration failed.")
		return err
	}

	applyRules := c.Render(table, chain, true)

	// apply correct rules
	for _, rule := range applyRules {
		cmd := append([]string{iptablesBin, "-t", string(table)}, strings.Fields(rule)...)
		if _, err = utils.RunCommand(ctx, cmd); err != nil {
			ReportError(ctx, err, "Firewall configuration failed.")
			return err
		}
	}

	ReportInfo(ctx, nil, "Load of new iptables rules succeeded for table %s.", table)

	return nil
}

func (c FirewallChain) renderRules(table FirewallTableName, chain FirewallChainName) []string {
	// for INPUT chain in the filter table we want to add some special rules
	rules := make([]string, 0)
	if table == Filter && chain == Input {
		rules = append(rules,
			"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
			"-A INPUT -i lo -j ACCEPT",
			// allow all traffic from the remote access network interface
			fmt.Sprintf("-A INPUT -i %s -j ACCEPT", remoteaccess.NetworkInterfaceName),
		)
	}

	for _, rule := range c.Rules {
		rules = append(rules, rule.Render(chain))
	}
	return rules
}

// Render rules based on provided firewall chain and table information.
func (c FirewallChain) Render(table FirewallTableName, chain FirewallChainName, policyLast bool) []string {
	policy := fmt.Sprintf("-P %s %s", chain, c.Policy)

	if policyLast {
		return append(c.renderRules(table, chain), policy)
	}
	return append([]string{policy}, c.renderRules(table, chain)...)
}

// FirewallRule defines a single firewall rule.
type FirewallRule struct {
	// SourceIP matches packets by source IP.
	SourceIP string `json:"srcIp"`

	// DestinationPort matches packets by destination port.
	DestinationPort string `json:"dstPort"`

	// Protocol matches packets by network protocol.
	Protocol Protocol `json:"proto"`

	// Target defines what to do with a packet when matched.
	Target Target `json:"target"`
}

// Render rule as a string acceptable by iptables.
func (r FirewallRule) Render(chain FirewallChainName) string {
	rule := []string{"-A", string(chain)}

	if r.SourceIP != "" && !strings.EqualFold(r.SourceIP, "any") {
		rule = append(rule, "-s", r.SourceIP)
	}

	rule = append(rule, "-p", string(r.Protocol), "-m", string(r.Protocol))

	if r.DestinationPort != "" && !strings.EqualFold(r.DestinationPort, "any") {
		rule = append(rule, "--dport", r.DestinationPort)
	}

	rule = append(rule, "-j", string(r.Target))

	return strings.Join(rule, " ")
}
