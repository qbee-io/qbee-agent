package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

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

const (
	Filter FirewallTableName = "filter"
	NAT    FirewallTableName = "nat"
)

// FirewallChainName defines firewall table's chain name.
type FirewallChainName string

const (
	Input       FirewallChainName = "INPUT"
	Forward     FirewallChainName = "FORWARD"
	Output      FirewallChainName = "OUTPUT"
	PreRouting  FirewallChainName = "PREROUTING"
	PostRouting FirewallChainName = "POSTROUTING"
)

// Protocol defines network protocol in use.
type Protocol string

const (
	TCP  Protocol = "tcp"
	UDP  Protocol = "udp"
	ICMP Protocol = "icmp"
)

// Target defines what to do with matching packets.
type Target string

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
	expectedRules := c.Render(table, chain)

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

	// apply correct rules
	for _, rule := range expectedRules {
		cmd := append([]string{iptablesBin, "-t", string(table)}, strings.Fields(rule)...)
		if _, err = utils.RunCommand(ctx, cmd); err != nil {
			ReportError(ctx, err, "Firewall configuration failed.")
			return err
		}
	}

	ReportInfo(ctx, nil, "Load of new iptables rules succeeded for table %s.", table)

	return nil
}

// Render rules based on provided firewall chain and table information.
func (c FirewallChain) Render(table FirewallTableName, chain FirewallChainName) []string {
	rules := []string{
		fmt.Sprintf("-P %s %s", chain, c.Policy),
	}

	// for INPUT chain in the filter table we want to add some special rules
	if table == Filter && chain == Input {
		rules = append(rules,
			"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
			"-A INPUT -i lo -j ACCEPT",
		)
	}

	for _, rule := range c.Rules {
		rules = append(rules, rule.Render(chain))
	}

	return rules
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

	if isNotFirewallAnyPortOrIPAddress(r.SourceIP) {
		rule = append(rule, "-s", r.SourceIP)
	}

	rule = append(rule, "-p", string(r.Protocol), "-m", string(r.Protocol))

	if isNotFirewallAnyPortOrIPAddress(r.DestinationPort) {
		rule = append(rule, "--dport", r.DestinationPort)
	}

	rule = append(rule, "-j", string(r.Target))

	return strings.Join(rule, " ")
}

func isNotFirewallAnyPortOrIPAddress(ipAddress string) bool {
	if ipAddress == "" {
		return false
	}

	if strings.EqualFold(ipAddress, "any") {
		return false
	}

	return true
}
