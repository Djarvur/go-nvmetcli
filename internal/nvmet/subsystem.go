package nvmet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Subsystem represents an NVMe Subsystem in configFS.
//
//nolint:govet // field alignment optimization is not critical for this struct
type Subsystem struct {
	CFSNode
	nqn string
}

// NewSubsystem creates a new Subsystem instance.
//
//nolint:gocritic // parameter type combination would reduce readability
func NewSubsystem(nqn string, mode string) (*Subsystem, error) {
	//nolint:exhaustruct,varnamelen,lll // CFSNode and nqn are initialized below; s is a standard short variable name for subsystem
	s := &Subsystem{}
	s.CFSNode.path = configfsDir
	s.attrGroups = []string{"attr"}

	if nqn == "" {
		if mode == "lookup" {
			return nil, &CFSError{Msg: "Need NQN for lookup"}
		}

		nqn = generateNQN()
	}

	// Validate NQN
	if err := validateNQN(nqn); err != nil {
		return nil, err
	}

	s.nqn = nqn
	s.path = filepath.Join(configfsDir, "subsystems", nqn)

	if err := s.createInCFS(mode); err != nil {
		return nil, err
	}

	return s, nil
}

func generateNQN() string {
	prefix := "nqn.2014-08.org.nvmexpress:NVMf:uuid"
	id := uuid.New()

	return fmt.Sprintf("%s:%s", prefix, id.String())
}

func validateNQN(nqn string) error {
	if nqn == "" {
		return &CFSError{Msg: "NQN cannot be empty"}
	}

	if strings.Contains(nqn, "/") {
		return &CFSError{Msg: "NQN cannot contain '/'"}
	}

	if len(nqn) > discoveryNQNMaxLen {
		return &CFSError{Msg: fmt.Sprintf("NQN too long (max %d)", discoveryNQNMaxLen)}
	}

	if nqn == discoveryNQN {
		return &CFSError{Msg: "Cannot use discovery NQN"}
	}

	return nil
}

// NQN returns the NQN of the subsystem.
func (s *Subsystem) NQN() string {
	return s.nqn
}

// Delete recursively deletes a Subsystem.
func (s *Subsystem) Delete() error {
	if err := s.checkSelf(); err != nil {
		return err
	}

	//nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
	namespaces, _ := s.Namespaces()
	for _, ns := range namespaces {
		//nolint:errcheck // Delete errors are non-critical during cleanup
		_ = ns.Delete()
	}

	//nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
	allowedHosts, _ := s.AllowedHosts()
	for _, host := range allowedHosts {
		//nolint:errcheck // RemoveAllowedHost errors are non-critical during cleanup
		_ = s.RemoveAllowedHost(host)
	}

	return s.CFSNode.Delete()
}

// Namespaces returns a list of namespaces for the subsystem.
func (s *Subsystem) Namespaces() ([]*Namespace, error) {
	if err := s.checkSelf(); err != nil {
		return nil, err
	}

	namespacesDir := filepath.Join(s.path, "namespaces")

	entries, err := os.ReadDir(namespacesDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var namespaces []*Namespace

	for _, entry := range entries {
		if entry.IsDir() {
			nsid, err := strconv.Atoi(entry.Name())
			if err == nil {
				ns, err := NewNamespace(s, nsid, "lookup")
				if err == nil {
					namespaces = append(namespaces, ns)
				}
			}
		}
	}

	return namespaces, nil
}

// AllowedHosts returns a list of allowed hosts for the subsystem.
func (s *Subsystem) AllowedHosts() ([]string, error) {
	if err := s.checkSelf(); err != nil {
		return nil, err
	}

	allowedHostsDir := filepath.Join(s.path, "allowed_hosts")

	entries, err := os.ReadDir(allowedHostsDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var hosts []string

	for _, entry := range entries {
		if entry.IsDir() || (entry.Type()&os.ModeSymlink != 0) {
			hosts = append(hosts, entry.Name())
		}
	}

	return hosts, nil
}

// AddAllowedHost enables access for a host identified by NQN.
func (s *Subsystem) AddAllowedHost(nqn string) error {
	if err := s.checkSelf(); err != nil {
		return err
	}

	target := filepath.Join(configfsDir, "hosts", nqn)
	link := filepath.Join(s.path, "allowed_hosts", nqn)

	// Check if target exists
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return &CFSError{Msg: fmt.Sprintf("Host %s does not exist", nqn)}
	}

	if err := os.Symlink(target, link); err != nil {
		return &CFSError{Msg: fmt.Sprintf("Could not symlink %s in configFS: %v", nqn, err)}
	}

	return nil
}

// RemoveAllowedHost disables access for a host identified by NQN.
func (s *Subsystem) RemoveAllowedHost(nqn string) error {
	if err := s.checkSelf(); err != nil {
		return err
	}

	link := filepath.Join(s.path, "allowed_hosts", nqn)
	if err := os.Remove(link); err != nil {
		return &CFSError{Msg: fmt.Sprintf("Could not unlink %s in configFS: %v", nqn, err)}
	}

	return nil
}

// SetupSubsystem sets up a Subsystem based on a map from saved config.
//
//nolint:cyclop // function complexity is necessary for subsystem setup logic
func SetupSubsystem(data map[string]interface{}, errFunc func(string)) error {
	nqn, ok := data["nqn"].(string)
	if !ok {
		errFunc("'nqn' not defined for Subsystem")

		return nil
	}

	subsys, err := NewSubsystem(nqn, "any")
	if err != nil {
		errFunc(fmt.Sprintf("Could not create Subsystem object: %v", err))

		return nil
	}

	// Setup namespaces
	if namespaces, ok := data["namespaces"].([]interface{}); ok {
		for _, nsData := range namespaces {
			if nsMap, ok := nsData.(map[string]interface{}); ok {
				if err := SetupNamespace(subsys, nsMap, errFunc); err != nil {
					errFunc(fmt.Sprintf("Error setting up namespace: %v", err))
				}
			}
		}
	}

	// Setup allowed hosts
	if allowedHosts, ok := data["allowed_hosts"].([]interface{}); ok {
		for _, host := range allowedHosts {
			if hostNQN, ok := host.(string); ok {
				if err := subsys.AddAllowedHost(hostNQN); err != nil {
					errFunc(fmt.Sprintf("Could not add allowed host %s: %v", hostNQN, err))
				}
			}
		}
	}

	subsys.setupAttrs(data, errFunc)

	return nil
}

// Dump returns a JSON-serializable representation of the subsystem.
func (s *Subsystem) Dump() (map[string]interface{}, error) {
	result, err := s.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	result["nqn"] = s.nqn

	namespaces, err := s.Namespaces()
	if err != nil {
		return nil, err
	}

	nsData := make([]interface{}, 0, len(namespaces))

	for _, ns := range namespaces {
		//nolint:govet // err shadowing is acceptable in loop context
		data, err := ns.Dump()
		if err != nil {
			return nil, err
		}

		nsData = append(nsData, data)
	}

	result["namespaces"] = nsData

	allowedHosts, err := s.AllowedHosts()
	if err != nil {
		return nil, err
	}

	result["allowed_hosts"] = allowedHosts

	return result, nil
}
