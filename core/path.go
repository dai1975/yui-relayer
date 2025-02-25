package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"

	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
)

const (
	check = "✔"
	xIcon = "✘"
)

// Paths represent connection paths between chains
type Paths map[string]*Path

// MustYAML returns the yaml string representation of the Paths
func (p Paths) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// Get returns the configuration for a given path
func (p Paths) Get(name string) (path *Path, err error) {
	if pth, ok := p[name]; ok {
		path = pth
	} else {
		err = fmt.Errorf("path with name %s does not exist", name)
	}
	return
}

// MustGet panics if path is not found
func (p Paths) MustGet(name string) *Path {
	pth, err := p.Get(name)
	if err != nil {
		panic(err)
	}
	return pth
}

// Add adds a path by its name
func (p Paths) Add(name string, path *Path) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if _, found := p[name]; found {
		return fmt.Errorf("path with name %s already exists", name)
	}
	p[name] = path
	return nil
}

// AddForce ignores existing paths and overwrites an existing path with that name
func (p Paths) AddForce(name string, path *Path) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if _, found := p[name]; found {
		fmt.Printf("overwriting path %s with new path...\n", name)
	}
	p[name] = path
	return nil
}

// MustYAML returns the yaml string representation of the Path
func (p *Path) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// PathsFromChains returns a path from the config between two chains
func (p Paths) PathsFromChains(src, dst string) (Paths, error) {
	out := Paths{}
	for name, path := range p {
		if (path.Dst.ChainID == src || path.Src.ChainID == src) && (path.Dst.ChainID == dst || path.Src.ChainID == dst) {
			out[name] = path
		}
	}
	if len(out) == 0 {
		return Paths{}, fmt.Errorf("failed to find path in config between chains %s and %s", src, dst)
	}
	return out, nil
}

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src      *PathEnd     `yaml:"src" json:"src"`
	Dst      *PathEnd     `yaml:"dst" json:"dst"`
	Strategy *StrategyCfg `yaml:"strategy" json:"strategy"`
}

// GenSrcClientID generates the specififed identifier
func (p *Path) GenSrcClientID() { p.Src.ClientID = RandLowerCaseLetterString(10) }

// GenDstClientID generates the specififed identifier
func (p *Path) GenDstClientID() { p.Dst.ClientID = RandLowerCaseLetterString(10) }

// GenSrcConnID generates the specififed identifier
func (p *Path) GenSrcConnID() { p.Src.ConnectionID = RandLowerCaseLetterString(10) }

// GenDstConnID generates the specififed identifier
func (p *Path) GenDstConnID() { p.Dst.ConnectionID = RandLowerCaseLetterString(10) }

// GenSrcChanID generates the specififed identifier
func (p *Path) GenSrcChanID() { p.Src.ChannelID = RandLowerCaseLetterString(10) }

// GenDstChanID generates the specififed identifier
func (p *Path) GenDstChanID() { p.Dst.ChannelID = RandLowerCaseLetterString(10) }

// Ordered returns true if the path is ordered and false if otherwise
func (p *Path) Ordered() bool {
	return p.Src.GetOrder() == chantypes.ORDERED
}

// Validate checks that a path is valid
func (p *Path) Validate() (err error) {
	if err = p.Src.Validate(); err != nil {
		return err
	}
	if p.Src.Version == "" {
		return fmt.Errorf("source must specify a version")
	}
	if err = p.Dst.Validate(); err != nil {
		return err
	}
	if err = p.ValidateStrategy(); err != nil {
		return err
	}
	if p.Src.Order != p.Dst.Order {
		return fmt.Errorf("both sides must have same order ('ORDERED' or 'UNORDERED'), got src(%s) and dst(%s)",
			p.Src.Order, p.Dst.Order)
	}
	return nil
}

// End returns the proper end given a chainID
func (p *Path) End(chainID string) *PathEnd {
	if p.Dst.ChainID == chainID {
		return p.Dst
	}
	if p.Src.ChainID == chainID {
		return p.Src
	}
	return &PathEnd{}
}

func (p *Path) String() string {
	return fmt.Sprintf("[ ] %s ->\n %s", p.Src.String(), p.Dst.String())
}

// GenPath generates a path with random client, connection and channel identifiers
// given chainIDs and portIDs
func GenPath(srcChainID, dstChainID, srcPortID, dstPortID, order string, version string) *Path {
	return &Path{
		Src: &PathEnd{
			ChainID:      srcChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       srcPortID,
			Order:        order,
			Version:      version,
		},
		Dst: &PathEnd{
			ChainID:      dstChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       dstPortID,
			Order:        order,
			Version:      version,
		},
		Strategy: &StrategyCfg{
			Type: "naive",
		},
	}
}

// PathStatus holds the status of the primatives in the path
type PathStatus struct {
	Chains     bool `yaml:"chains" json:"chains"`
	Clients    bool `yaml:"clients" json:"clients"`
	Connection bool `yaml:"connection" json:"connection"`
	Channel    bool `yaml:"channel" json:"channel"`
}

// PathWithStatus is used for showing the status of the path
type PathWithStatus struct {
	Path   *Path      `yaml:"path" json:"chains"`
	Status PathStatus `yaml:"status" json:"status"`
}

// QueryPathStatus returns an instance of the path struct with some attached data about
// the current status of the path
func (p *Path) QueryPathStatus(src, dst *ProvableChain) *PathWithStatus {
	var (
		err              error
		eg               errgroup.Group
		srch, dsth       ibcexported.Height
		srcCs, dstCs     *clienttypes.QueryClientStateResponse
		srcConn, dstConn *conntypes.QueryConnectionResponse
		srcChan, dstChan *chantypes.QueryChannelResponse

		out = &PathWithStatus{Path: p, Status: PathStatus{false, false, false, false}}
	)
	eg.Go(func() error {
		srch, err = src.LatestHeight()
		return err
	})
	eg.Go(func() error {
		dsth, err = dst.LatestHeight()
		return err
	})
	if eg.Wait(); err != nil {
		return out
	}
	out.Status.Chains = true

	ctx := context.TODO()

	eg.Go(func() error {
		srcCs, err = src.QueryClientState(NewQueryContext(ctx, srch))
		return err
	})
	eg.Go(func() error {
		dstCs, err = dst.QueryClientState(NewQueryContext(ctx, dsth))
		return err
	})
	if err = eg.Wait(); err != nil || srcCs == nil || dstCs == nil {
		return out
	}
	out.Status.Clients = true

	eg.Go(func() error {
		srcConn, err = src.QueryConnection(NewQueryContext(ctx, srch))
		return err
	})
	eg.Go(func() error {
		dstConn, err = dst.QueryConnection(NewQueryContext(ctx, dsth))
		return err
	})
	if err = eg.Wait(); err != nil || srcConn.Connection.State != conntypes.OPEN || dstConn.Connection.State != conntypes.OPEN {
		return out
	}
	out.Status.Connection = true

	eg.Go(func() error {
		srcChan, err = src.QueryChannel(NewQueryContext(ctx, srch))
		return err
	})
	eg.Go(func() error {
		dstChan, err = dst.QueryChannel(NewQueryContext(ctx, dsth))
		return err
	})
	if err = eg.Wait(); err != nil || srcChan.Channel.State != chantypes.OPEN || dstChan.Channel.State != chantypes.OPEN {
		return out
	}
	out.Status.Channel = true
	return out
}

// PrintString prints a string representations of the path status
func (ps *PathWithStatus) PrintString(name string) string {
	pth := ps.Path
	return fmt.Sprintf(`Path "%s" strategy(%s):
  SRC(%s)
    ClientID:     %s
    ConnectionID: %s
    ChannelID:    %s
    PortID:       %s
  DST(%s)
    ClientID:     %s
    ConnectionID: %s
    ChannelID:    %s
    PortID:       %s
  STATUS:
    Chains:       %s
    Clients:      %s
    Connection:   %s
    Channel:      %s`, name, pth.Strategy.Type, pth.Src.ChainID,
		pth.Src.ClientID, pth.Src.ConnectionID, pth.Src.ChannelID, pth.Src.PortID,
		pth.Dst.ChainID, pth.Dst.ClientID, pth.Dst.ConnectionID, pth.Dst.ChannelID, pth.Dst.PortID,
		checkmark(ps.Status.Chains), checkmark(ps.Status.Clients), checkmark(ps.Status.Connection), checkmark(ps.Status.Channel))
}

func checkmark(status bool) string {
	if status {
		return check
	}
	return xIcon
}

// RandLowerCaseLetterString returns a lowercase letter string of given length
func RandLowerCaseLetterString(length int) string {
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	var b strings.Builder
	for i := 0; i < length; i++ {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteRune(chars[i.Int64()])
	}
	return b.String()
}
