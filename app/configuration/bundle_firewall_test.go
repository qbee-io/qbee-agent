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

package configuration_test

import (
	"reflect"
	"strings"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Firewall_NoIPTablesInstalled(t *testing.T) {
	r := runner.New(t)

	reports := executeFirewallBundle(r, configuration.FirewallBundle{
		Tables: map[configuration.FirewallTableName]configuration.FirewallTable{
			configuration.Filter: {
				configuration.Input: configuration.FirewallChain{
					Policy: configuration.Drop,
				},
			},
		},
	})

	expectedReports := []string{
		"[ERR] Firewall configuration failed.",
	}

	assert.Equal(t, reports, expectedReports)
}

func Test_Firewall(t *testing.T) {
	r := runner.New(t)

	r.MustExec("apt-get", "install", "-y", "iptables")

	firewallBundle := configuration.FirewallBundle{
		Tables: map[configuration.FirewallTableName]configuration.FirewallTable{
			configuration.Filter: {
				configuration.Input: configuration.FirewallChain{
					Policy: configuration.Drop,
					Rules: []configuration.FirewallRule{
						// both source IP and destination port
						{
							SourceIP:        "1.1.1.1/32",
							DestinationPort: "123",
							Protocol:        configuration.TCP,
							Target:          configuration.Accept,
						},
						// only source IP
						{
							SourceIP: "2.2.2.2/32",
							Protocol: configuration.UDP,
							Target:   configuration.Drop,
						},
						// only destination port
						{
							DestinationPort: "333",
							Protocol:        configuration.TCP,
							Target:          configuration.Accept,
						},
					},
				},
			},
		},
	}

	// check that the first run changes the firewall
	reports := executeFirewallBundle(r, firewallBundle)
	expectedReports := []string{
		"[WARN] Current firewall rules are not in compliance.",
		"[INFO] Load of new iptables rules succeeded for table filter.",
	}

	assert.Equal(t, reports, expectedReports)

	// check that correct rules are set on the filter/INPUT
	output := r.MustExec("iptables", "-t", "filter", "-S", "INPUT")

	gotRules := strings.Split(string(output), "\n")
	expectedRules := []string{
		"-P INPUT DROP",
		"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
		"-A INPUT -i lo -j ACCEPT",
		"-A INPUT -i qbee0 -j ACCEPT",
		"-A INPUT -s 1.1.1.1/32 -p tcp -m tcp --dport 123 -j ACCEPT",
		"-A INPUT -s 2.2.2.2/32 -p udp -m udp -j DROP",
		"-A INPUT -p tcp -m tcp --dport 333 -j ACCEPT",
	}

	assert.Equal(t, gotRules, expectedRules)

	// check that the second run doesn't change the firewall
	reports = executeFirewallBundle(r, firewallBundle)
	assert.Empty(t, reports)
}

// executeFirewallBundle is a helper method to quickly execute firewall bundle.
// On success, it returns a slice of produced reports.
func executeFirewallBundle(r *runner.Runner, bundle configuration.FirewallBundle) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFirewall},
		BundleData: configuration.BundleData{
			Firewall: &bundle,
		},
	}

	config.BundleData.Firewall.Enabled = true

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}

func TestFirewallChain_Render(t *testing.T) {
	tests := []struct {
		name      string
		chain     configuration.FirewallChain
		tableName configuration.FirewallTableName
		chainName configuration.FirewallChainName
		want      []string
	}{
		{
			name: "filter / INPUT extra rules",
			chain: configuration.FirewallChain{
				Policy: configuration.Drop,
				Rules: []configuration.FirewallRule{
					{
						SourceIP:        "1.1.1.1/32",
						DestinationPort: "123",
						Protocol:        configuration.TCP,
						Target:          configuration.Accept,
					},
				},
			},
			tableName: configuration.Filter,
			chainName: configuration.Input,
			want: []string{
				"-P INPUT DROP",
				"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
				"-A INPUT -i lo -j ACCEPT",
				"-A INPUT -i qbee0 -j ACCEPT",
				"-A INPUT -s 1.1.1.1/32 -p tcp -m tcp --dport 123 -j ACCEPT",
			},
		},
		{
			name: "filter / OUTPUT without extra rules",
			chain: configuration.FirewallChain{
				Policy: configuration.Drop,
			},
			tableName: configuration.Filter,
			chainName: configuration.Output,
			want: []string{
				"-P OUTPUT DROP",
			},
		},
		{
			name: "only source IP",
			chain: configuration.FirewallChain{
				Policy: configuration.Drop,
				Rules: []configuration.FirewallRule{
					{
						SourceIP: "1.1.1.1/32",
						Protocol: configuration.TCP,
						Target:   configuration.Accept,
					},
				},
			},
			tableName: configuration.Filter,
			chainName: configuration.Output,
			want: []string{
				"-P OUTPUT DROP",
				"-A OUTPUT -s 1.1.1.1/32 -p tcp -m tcp -j ACCEPT",
			},
		},
		{
			name: "only destination port",
			chain: configuration.FirewallChain{
				Policy: configuration.Drop,
				Rules: []configuration.FirewallRule{
					{
						DestinationPort: "123",
						Protocol:        configuration.TCP,
						Target:          configuration.Accept,
					},
				},
			},
			tableName: configuration.Filter,
			chainName: configuration.Output,
			want: []string{
				"-P OUTPUT DROP",
				"-A OUTPUT -p tcp -m tcp --dport 123 -j ACCEPT",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chain.Render(tt.tableName, tt.chainName, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}
