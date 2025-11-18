package nvmet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Port represents an NVMe Port in configFS.
type Port struct {
	CFSNode
	portid int
}

// NewPort creates a new Port instance.
func NewPort(portid int, mode string) (*Port, error) {
	//nolint:exhaustruct,varnamelen,lll // CFSNode and portid are initialized below; p is a standard short variable name for port
	p := &Port{}
	p.CFSNode.path = configfsDir
	p.attrGroups = []string{"addr"}

	if portid < 1 || portid > maxPortID {
		return nil, &CFSError{Msg: fmt.Sprintf("Port ID must be 1 to %d", maxPortID)}
	}

	p.portid = portid
	p.path = filepath.Join(configfsDir, "ports", strconv.Itoa(portid))

	if err := p.createInCFS(mode); err != nil {
		return nil, err
	}

	return p, nil
}

// PortID returns the port ID.
func (p *Port) PortID() int {
	return p.portid
}

// Subsystems returns a list of subsystems for this port.
func (p *Port) Subsystems() ([]string, error) {
	if err := p.checkSelf(); err != nil {
		return nil, err
	}

	subsystemsDir := filepath.Join(p.path, "subsystems")

	entries, err := os.ReadDir(subsystemsDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var subsystems []string

	for _, entry := range entries {
		if entry.IsDir() || (entry.Type()&os.ModeSymlink != 0) {
			subsystems = append(subsystems, entry.Name())
		}
	}

	return subsystems, nil
}

// AddSubsystem enables access to a subsystem identified by NQN through this port.
func (p *Port) AddSubsystem(nqn string) error {
	if err := p.checkSelf(); err != nil {
		return err
	}

	target := filepath.Join(configfsDir, "subsystems", nqn)
	link := filepath.Join(p.path, "subsystems", nqn)

	// Check if target exists
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return &CFSError{Msg: fmt.Sprintf("Subsystem %s does not exist", nqn)}
	}

	if err := os.Symlink(target, link); err != nil {
		return &CFSError{Msg: fmt.Sprintf("Could not symlink %s in configFS: %v", nqn, err)}
	}

	return nil
}

// RemoveSubsystem disables access to a subsystem identified by NQN through this port.
func (p *Port) RemoveSubsystem(nqn string) error {
	if err := p.checkSelf(); err != nil {
		return err
	}

	link := filepath.Join(p.path, "subsystems", nqn)
	if err := os.Remove(link); err != nil {
		return &CFSError{Msg: fmt.Sprintf("Could not unlink %s in configFS: %v", nqn, err)}
	}

	return nil
}

// Delete recursively deletes a Port.
func (p *Port) Delete() error {
	if err := p.checkSelf(); err != nil {
		return err
	}

	//nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
	subsystems, _ := p.Subsystems()
	for _, s := range subsystems {
		//nolint:errcheck // RemoveSubsystem errors are non-critical during cleanup
		_ = p.RemoveSubsystem(s)
	}

	//nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
	referrals, _ := p.Referrals()
	for _, r := range referrals {
		//nolint:errcheck // Delete errors are non-critical during cleanup
		_ = r.Delete()
	}

	return p.CFSNode.Delete()
}

// Referrals returns a list of referrals for this port.
func (p *Port) Referrals() ([]*Referral, error) {
	if err := p.checkSelf(); err != nil {
		return nil, err
	}

	referralsDir := filepath.Join(p.path, "referrals")

	entries, err := os.ReadDir(referralsDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var referrals []*Referral

	for _, entry := range entries {
		if entry.IsDir() {
			ref, err := NewReferral(p, entry.Name(), "lookup")
			if err == nil {
				referrals = append(referrals, ref)
			}
		}
	}

	return referrals, nil
}

// SetupPort sets up a Port based on a map from saved config.
//
//nolint:cyclop // function complexity is necessary for port setup logic
func SetupPort(_ *Root, data map[string]interface{}, errFunc func(string)) error {
	var (
		portid int
		//nolint:varnamelen // ok is a standard Go idiom for type assertion result
		ok bool
	)

	switch v := data["portid"].(type) {
	case int:
		portid = v
		ok = true
	case float64:
		portid = int(v)
		ok = true
	}

	if !ok {
		errFunc("'portid' not defined for Port")

		return nil
	}

	port, err := NewPort(portid, "any")
	if err != nil {
		errFunc(fmt.Sprintf("Could not create Port object: %v", err))

		return nil
	}

	port.setupAttrs(data, errFunc)

	// Add subsystems
	if subsystems, ok := data["subsystems"].([]interface{}); ok {
		for _, subsys := range subsystems {
			if nqn, ok := subsys.(string); ok {
				if err := port.AddSubsystem(nqn); err != nil {
					errFunc(fmt.Sprintf("Could not add subsystem %s: %v", nqn, err))
				}
			}
		}
	}

	// Setup referrals
	if referrals, ok := data["referrals"].([]interface{}); ok {
		for _, refData := range referrals {
			if refMap, ok := refData.(map[string]interface{}); ok {
				if err := SetupReferral(port, refMap, errFunc); err != nil {
					errFunc(fmt.Sprintf("Error setting up referral: %v", err))
				}
			}
		}
	}

	return nil
}

// Dump returns a JSON-serializable representation of the port.
func (p *Port) Dump() (map[string]interface{}, error) {
	result, err := p.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	result["portid"] = p.portid

	subsystems, err := p.Subsystems()
	if err != nil {
		return nil, err
	}

	result["subsystems"] = subsystems

	referrals, err := p.Referrals()
	if err != nil {
		return nil, err
	}

	refData := make([]interface{}, 0, len(referrals))

	for _, r := range referrals {
		data, err := r.Dump()
		if err != nil {
			return nil, err
		}

		refData = append(refData, data)
	}

	result["referrals"] = refData

	return result, nil
}
