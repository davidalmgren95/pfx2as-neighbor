package bgpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/server"
)

type BgpServer struct {
	s       *server.BgpServer
	paths   map[string]uint32 // prefix -> announced origin AS, for change detection
	nextHop string
}

type Neighbor struct {
	Address      string `yaml:"address"`
	ASN          uint32 `yaml:"asn"`
	Password     string `yaml:"password"`
	ForcePrepend bool   `yaml:"force_prepend"`
}

type Config struct {
	ASN            uint32     `yaml:"asn"`
	RouterID       string     `yaml:"router_id"`
	ListenPort     int        `yaml:"listen_port"`
	HoldTimer      int        `yaml:"hold_timer"`
	UpdateInterval string     `yaml:"update_interval"`
	Neighbors      []Neighbor `yaml:"neighbors"`
}

func Start(ctx context.Context, cfg *Config) (*BgpServer, error) {
	s := server.NewBgpServer()
	go s.Serve()

	err := s.StartBgp(ctx, &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        cfg.ASN,
			RouterId:   cfg.RouterID,
			ListenPort: int32(cfg.ListenPort),
		},
	})
	if err != nil {
		return nil, err
	}
	slog.Info("BGP server started", "asn", cfg.ASN, "router_id", cfg.RouterID, "listen_port", cfg.ListenPort)

	for _, neighbor := range cfg.Neighbors {
		err := s.AddPeer(ctx, &api.AddPeerRequest{
			Peer: &api.Peer{
				Conf: &api.PeerConf{
					NeighborAddress: neighbor.Address,
					PeerAsn:         neighbor.ASN,
					AuthPassword:    neighbor.Password,
				},
			},
		})
		if err != nil {
			return nil, err
		}
		sessionType := "eBGP"
		if neighbor.ASN == cfg.ASN {
			sessionType = "iBGP"
		}
		slog.Info("neighbor added",
			"address", neighbor.Address,
			"asn", neighbor.ASN,
			"session_type", sessionType,
			"force_prepend", neighbor.ForcePrepend,
		)
	}

	if err := addForcePrependPolicy(ctx, s, cfg); err != nil {
		return nil, err
	}

	return &BgpServer{
		s:       s,
		paths:   make(map[string]uint32),
		nextHop: cfg.RouterID,
	}, nil
}

func addForcePrependPolicy(ctx context.Context, s *server.BgpServer, cfg *Config) error {
	var addrs []string
	for _, n := range cfg.Neighbors {
		if n.ForcePrepend {
			addr, err := netip.ParseAddr(n.Address)
			if err != nil {
				return fmt.Errorf("invalid neighbor address %q: %v", n.Address, err)
			}
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			addrs = append(addrs, fmt.Sprintf("%s/%d", addr, bits))
		}
	}
	if len(addrs) == 0 {
		return nil
	}

	const setName = "force-prepend-neighbors"
	const policyName = "force-prepend-policy"
	const stmtName = "force-prepend-stmt"

	err := s.AddDefinedSet(ctx, &api.AddDefinedSetRequest{
		DefinedSet: &api.DefinedSet{
			DefinedType: api.DefinedType_DEFINED_TYPE_NEIGHBOR,
			Name:        setName,
			List:        addrs,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add force-prepend neighbor set: %v", err)
	}

	err = s.AddStatement(ctx, &api.AddStatementRequest{
		Statement: &api.Statement{
			Name: stmtName,
			Conditions: &api.Conditions{
				NeighborSet: &api.MatchSet{
					Type: api.MatchSet_TYPE_ANY,
					Name: setName,
				},
			},
			Actions: &api.Actions{
				RouteAction: api.RouteAction_ROUTE_ACTION_ACCEPT,
				AsPrepend: &api.AsPrependAction{
					Asn:    cfg.ASN,
					Repeat: 1,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add force-prepend statement: %v", err)
	}

	err = s.AddPolicy(ctx, &api.AddPolicyRequest{
		Policy: &api.Policy{
			Name:       policyName,
			Statements: []*api.Statement{{Name: stmtName}},
		},
		ReferExistingStatements: true,
	})
	if err != nil {
		return fmt.Errorf("failed to add force-prepend policy: %v", err)
	}

	err = s.AddPolicyAssignment(ctx, &api.AddPolicyAssignmentRequest{
		Assignment: &api.PolicyAssignment{
			Name:          "",
			Direction:     api.PolicyDirection_POLICY_DIRECTION_EXPORT,
			Policies:      []*api.Policy{{Name: policyName}},
			DefaultAction: api.RouteAction_ROUTE_ACTION_ACCEPT,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to assign force-prepend policy: %v", err)
	}

	slog.Info("force-prepend export policy configured", "neighbors", addrs)
	return nil
}

func (b *BgpServer) Stop(ctx context.Context) error {
	return b.s.StopBgp(ctx, &api.StopBgpRequest{})
}

// AddPath installs or updates a prefix. Returns (true, nil) if the path was
// added or updated, (false, nil) if it was already current, or (false, err).
func (b *BgpServer) AddPath(prefix string, asn uint32) (bool, error) {
	if existing, ok := b.paths[prefix]; ok {
		if existing == asn {
			return false, nil
		}
		if err := b.DeletePath(prefix); err != nil {
			return false, fmt.Errorf("failed to replace path for %s: %v", prefix, err)
		}
	}

	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return false, fmt.Errorf("invalid prefix %s: %v", prefix, err)
	}

	nlri, err := bgp.NewIPAddrPrefix(p)
	if err != nil {
		return false, fmt.Errorf("failed to create NLRI for prefix %s: %v", prefix, err)
	}

	nextHop, err := bgp.NewPathAttributeNextHop(netip.MustParseAddr(b.nextHop))
	if err != nil {
		return false, fmt.Errorf("invalid next-hop %s: %v", b.nextHop, err)
	}

	// Single best origin AS, so always an AS_SEQUENCE (never an AS_SET).
	_, err = b.s.AddPath(apiutil.AddPathRequest{
		Paths: []*apiutil.Path{
			{
				Family: bgp.RF_IPv4_UC,
				Nlri:   nlri,
				Attrs: []bgp.PathAttributeInterface{
					bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
					nextHop,
					bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
						bgp.NewAs4PathParam(uint8(bgp.BGP_ASPATH_ATTR_TYPE_SEQ), []uint32{asn}),
					}),
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("failed to add path for prefix %s: %v", prefix, err)
	}

	b.paths[prefix] = asn
	return true, nil
}

func (b *BgpServer) DeletePath(prefix string) error {
	if _, ok := b.paths[prefix]; !ok {
		return fmt.Errorf("no path found for prefix: %s", prefix)
	}

	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return fmt.Errorf("invalid prefix %s: %v", prefix, err)
	}

	nlri, err := bgp.NewIPAddrPrefix(p)
	if err != nil {
		return fmt.Errorf("failed to create NLRI for prefix %s: %v", prefix, err)
	}

	err = b.s.DeletePath(apiutil.DeletePathRequest{
		Paths: []*apiutil.Path{
			{
				Family:     bgp.RF_IPv4_UC,
				Nlri:       nlri,
				Withdrawal: true, // skip GoBGP's nexthop-required check; this is a withdrawal
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete path for prefix %s: %v", prefix, err)
	}

	delete(b.paths, prefix)
	return nil
}

func (b *BgpServer) ActivePrefixes() map[string]struct{} {
	out := make(map[string]struct{}, len(b.paths))
	for k := range b.paths {
		out[k] = struct{}{}
	}
	return out
}
