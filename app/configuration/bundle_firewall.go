package configuration

// Firewall configures system firewall.
//
// Example payload:
// {
//  "tables": {
//    "filter": {
//      "INPUT": {
//        "policy": "ACCEPT",
//        "rules": [
//          {
//            "srcIp": "192.168.1.1",
//            "dstPort": "80",
//            "proto": "tcp",
//            "target": "ACCEPT"
//          }
//        ]
//      }
//    }
//  }
// }
type Firewall struct {
	Metadata

	// Tables defines a map of firewall tables to be modified.
	Tables map[FirewallTableName]FirewallTable `json:"tables"`
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
