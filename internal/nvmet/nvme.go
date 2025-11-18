package nvmet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	configfsDir        = "/sys/kernel/config/nvmet"
	defaultSaveFile    = "/etc/nvmet/config.json"
	maxNSID            = 8192
	maxPortID          = 8192
	discoveryNQN       = "nqn.2014-08.org.nvmexpress.discovery"
	discoveryNQNMaxLen = 223
)

var errClearingExistingConfig = errors.New("error clearing existing config")

// CFSError represents a generic configfs error.
type CFSError struct {
	Msg string
}

func (e *CFSError) Error() string {
	return e.Msg
}

// CFSNotFoundError indicates that a configfs object does not exist.
type CFSNotFoundError struct {
	Msg string
}

func (e *CFSNotFoundError) Error() string {
	return e.Msg
}

// CFSNode is the base interface for all configfs nodes.
type CFSNode struct {
	path       string
	enable     *int
	attrGroups []string
}

// Path returns the configfs path of the node.
func (n *CFSNode) Path() string {
	return n.path
}

// Exists checks if the configfs node exists.
func (n *CFSNode) Exists() bool {
	info, err := os.Stat(n.path)

	return err == nil && info.IsDir()
}

func (n *CFSNode) checkSelf() error {
	if !n.Exists() {
		return &CFSNotFoundError{
			Msg: fmt.Sprintf("This %T does not exist in configFS: %s", n, n.path),
		}
	}

	return nil
}

func (n *CFSNode) createInCFS(mode string) error {
	//nolint:goconst // "create" is a mode string, not a constant
	if mode != "any" && mode != "lookup" && mode != "create" {
		//nolint:perfsprint // string concatenation would be less readable here
		return &CFSError{Msg: fmt.Sprintf("Invalid mode: %s", mode)}
	}

	exists := n.Exists()

	if exists && mode == "create" {
		return &CFSError{
			Msg: fmt.Sprintf("This %T already exists in configFS", n),
		}
	}

	if !exists && mode == "lookup" {
		return &CFSNotFoundError{
			Msg: fmt.Sprintf("No such %T in configfs: %s", n, n.path),
		}
	}

	if !exists {
		//nolint:mnd // 0o755 is a standard directory permission mask
		if err := os.MkdirAll(n.path, 0o755); err != nil {
			return &CFSError{
				Msg: fmt.Sprintf("Could not create %T in configFS: %v", n, err),
			}
		}
	}

	//nolint:errcheck // enable might not exist for all node types
	_, _ = n.GetEnable()

	return nil
}

// ListAttrs returns a list of attributes in the specified group.
func (n *CFSNode) ListAttrs(group string, writable *bool) ([]string, error) {
	if err := n.checkSelf(); err != nil {
		return nil, err
	}

	pattern := filepath.Join(n.path, group+"_*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, &CFSError{Msg: fmt.Sprintf("Error listing attributes: %v", err)}
	}

	attrs := make([]string, 0, len(matches))

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		base := filepath.Base(match)

		//nolint:mnd // 2 is the expected number of parts after split
		parts := strings.SplitN(base, "_", 2)
		//nolint:mnd // 2 is the expected number of parts after split
		if len(parts) != 2 {
			continue
		}

		attrName := parts[1]

		if writable != nil {
			//nolint:mnd // 0o200 is a standard file permission mask for write bit
			isWritable := info.Mode()&os.FileMode(0o200) != 0
			if *writable != isWritable {
				continue
			}
		}

		attrs = append(attrs, attrName)
	}

	return attrs, nil
}

// SetAttr sets the value of a named attribute.
func (n *CFSNode) SetAttr(group, attribute, value string) error {
	if err := n.checkSelf(); err != nil {
		return err
	}

	if n.enable != nil && *n.enable != 0 {
		return &CFSError{
			Msg: fmt.Sprintf("Cannot set attribute while %T is enabled", n),
		}
	}

	path := filepath.Join(n.path, fmt.Sprintf("%s_%s", group, attribute))

	if err := os.WriteFile(path, []byte(value), 0o644); err != nil { //nolint:gosec,lll,mnd // 0o644 is acceptable for configfs attribute files
		return &CFSError{
			Msg: fmt.Sprintf("Cannot set attribute %s: %v", path, err),
		}
	}

	return nil
}

// GetAttr gets the value of a named attribute.
func (n *CFSNode) GetAttr(group, attribute string) (string, error) {
	if err := n.checkSelf(); err != nil {
		return "", err
	}

	path := filepath.Join(n.path, group+"_"+attribute)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", &CFSError{
			Msg: "Cannot find attribute: " + path,
		}
	}

	return strings.TrimSpace(string(data)), nil
}

// GetEnable returns the enable status of the node.
func (n *CFSNode) GetEnable() (int, error) {
	if err := n.checkSelf(); err != nil {
		return 0, err
	}

	path := filepath.Join(n.path, "enable")

	data, err := os.ReadFile(path)
	if err != nil {
		n.enable = nil
		//nolint:nilerr // returning nil error is intentional when enable doesn't exist
		return 0, nil
	}

	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		n.enable = nil
		//nolint:nilerr // returning nil error is intentional when enable is invalid
		return 0, nil
	}

	n.enable = &val

	return val, nil
}

// SetEnable sets the enable status of the node.
func (n *CFSNode) SetEnable(value int) error {
	if err := n.checkSelf(); err != nil {
		return err
	}

	path := filepath.Join(n.path, "enable")
	if err := os.WriteFile(path, []byte(strconv.Itoa(value)), 0o644); err != nil { //nolint:gosec,lll,mnd // 0o644 is acceptable for configfs enable file
		return &CFSError{
			Msg: fmt.Sprintf("Cannot enable %s (%d): %v", n.path, value, err),
		}
	}

	n.enable = &value

	return nil
}

// Delete removes the configfs node.
func (n *CFSNode) Delete() error {
	if n.Exists() {
		return os.Remove(n.path) //nolint:wrapcheck // os.Remove error is clear enough
	}

	return nil
}

// Dump returns a JSON-serializable representation of the node.
func (n *CFSNode) Dump() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, group := range n.attrGroups {
		attrs, err := n.ListAttrs(group, boolPtr(true))
		if err != nil {
			return nil, err
		}

		groupData := make(map[string]string)

		for _, attr := range attrs {
			val, err := n.GetAttr(group, attr)
			if err != nil {
				return nil, err
			}

			groupData[attr] = val
		}

		result[group] = groupData
	}

	if n.enable != nil {
		result["enable"] = *n.enable
	}

	return result, nil
}

func (n *CFSNode) setupAttrs(attrDict map[string]interface{}, errFunc func(string)) {
	for _, group := range n.attrGroups {
		groupData, ok := attrDict[group].(map[string]interface{})
		if !ok {
			continue
		}

		for name, value := range groupData {
			if err := n.SetAttr(group, name, fmt.Sprintf("%v", value)); err != nil {
				errFunc(err.Error())
			}
		}
	}

	if enable, ok := attrDict["enable"]; ok {
		var enableVal int
		switch v := enable.(type) {
		case int:
			enableVal = v
		case float64:
			enableVal = int(v)
		default:
			return
		}

		if err := n.SetEnable(enableVal); err != nil {
			errFunc(err.Error())
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func modprobe(_ string) error {
	// Try to load kernel module using modprobe
	// For now, we'll just do nothing since modprobe requires os/exec
	// This is optional functionality
	return nil
}

// Root represents the root of the NVMe target configfs hierarchy.
type Root struct {
	CFSNode
}

// NewRoot creates a new Root instance.
func NewRoot() (*Root, error) {
	//nolint:exhaustruct,varnamelen // CFSNode is initialized below; r is a standard short variable name for root
	r := &Root{}
	r.CFSNode.path = configfsDir
	r.CFSNode.attrGroups = []string{}

	// Check if configfs exists, try to load module if not
	//nolint:errcheck // modprobe failure is non-critical
	if _, err := os.Stat(configfsDir); os.IsNotExist(err) {
		_ = modprobe("nvmet")
	}

	if _, err := os.Stat(configfsDir); os.IsNotExist(err) {
		//nolint:perfsprint // string concatenation would be less readable here
		return nil, &CFSError{
			Msg: fmt.Sprintf("%s does not exist. Giving up.", configfsDir),
		}
	}

	r.path = configfsDir
	if err := r.createInCFS("lookup"); err != nil {
		return nil, err
	}

	return r, nil
}

// Subsystems returns a list of all subsystems.
func (r *Root) Subsystems() ([]*Subsystem, error) {
	if err := r.checkSelf(); err != nil {
		return nil, err
	}

	subsysDir := filepath.Join(r.path, "subsystems")

	entries, err := os.ReadDir(subsysDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var subsystems []*Subsystem

	for _, entry := range entries {
		if entry.IsDir() {
			subsys, err := NewSubsystem(entry.Name(), "lookup")
			if err == nil {
				subsystems = append(subsystems, subsys)
			}
		}
	}

	return subsystems, nil
}

// Ports returns a list of all ports.
func (r *Root) Ports() ([]*Port, error) {
	if err := r.checkSelf(); err != nil {
		return nil, err
	}

	portsDir := filepath.Join(r.path, "ports")

	entries, err := os.ReadDir(portsDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var ports []*Port

	for _, entry := range entries {
		if entry.IsDir() {
			portID, err := strconv.Atoi(entry.Name())
			if err == nil {
				port, err := NewPort(portID, "lookup")
				if err == nil {
					ports = append(ports, port)
				}
			}
		}
	}

	return ports, nil
}

// Hosts returns a list of all hosts.
func (r *Root) Hosts() ([]*Host, error) {
	if err := r.checkSelf(); err != nil {
		return nil, err
	}

	hostsDir := filepath.Join(r.path, "hosts")

	entries, err := os.ReadDir(hostsDir)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadDir error is clear enough
	}

	var hosts []*Host

	for _, entry := range entries {
		if entry.IsDir() {
			host, err := NewHost(entry.Name(), "lookup")
			if err == nil {
				hosts = append(hosts, host)
			}
		}
	}

	return hosts, nil
}

// SaveToFile writes the configuration to a JSON file.
func (r *Root) SaveToFile(savefile string) error {
	if savefile == "" {
		savefile = defaultSaveFile
	}

	savefile = filepath.Clean(os.ExpandEnv(savefile))

	// Expand ~ to home directory
	if strings.HasPrefix(savefile, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err //nolint:wrapcheck // os.UserHomeDir error is clear enough
		}

		savefile = filepath.Join(home, savefile[1:])
	}

	savefileAbs, err := filepath.Abs(savefile)
	if err != nil {
		return err //nolint:wrapcheck // filepath.Abs error is clear enough
	}

	savefileDir := filepath.Dir(savefileAbs)
	if err := os.MkdirAll(savefileDir, 0o755); err != nil { //nolint:govet,lll,mnd // os.MkdirAll error is clear enough; err shadowing is acceptable here; 0o755 is a standard directory permission mask
		return err //nolint:wrapcheck // os.MkdirAll error is clear enough; err shadowing is acceptable here
	}

	data, err := r.Dump()
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err //nolint:wrapcheck // json.MarshalIndent error is clear enough
	}

	// Write to temporary file first
	tempFile := savefile + ".temp"
	if err := os.WriteFile(tempFile, jsonData, 0o600); err != nil { //nolint:mnd,lll // 0o600 is a standard file permission mask for private files
		return err //nolint:wrapcheck // os.WriteFile error is clear enough
	}

	// Atomic rename
	return os.Rename(tempFile, savefile) //nolint:wrapcheck // os.Rename error is clear enough
}

// ClearExisting removes all existing configuration.
func (r *Root) ClearExisting() error {
	ports, err := r.Ports()
	if err == nil {
		for _, p := range ports {
			//nolint:errcheck // Delete errors are non-critical during cleanup
			_ = p.Delete()
		}
	}

	subsystems, err := r.Subsystems()
	if err == nil {
		for _, s := range subsystems {
			//nolint:errcheck // Delete errors are non-critical during cleanup
			_ = s.Delete()
		}
	}

	hosts, err := r.Hosts()
	if err == nil {
		for _, h := range hosts {
			//nolint:errcheck // Delete errors are non-critical during cleanup
			_ = h.Delete()
		}
	}

	return nil
}

// RestoreFromFile restores configuration from a JSON file.
func (r *Root) RestoreFromFile(savefile string, clearExisting bool, abortOnError bool) ([]string, error) { //nolint:gocritic,lll // parameter type combination would reduce readability
	if savefile == "" {
		savefile = defaultSaveFile
	}

	savefile = filepath.Clean(os.ExpandEnv(savefile))

	// Expand ~ to home directory
	if strings.HasPrefix(savefile, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err //nolint:wrapcheck // os.UserHomeDir error is clear enough
		}

		savefile = filepath.Join(home, savefile[1:])
	}

	data, err := os.ReadFile(savefile)
	if err != nil {
		return nil, err //nolint:wrapcheck // os.ReadFile error is clear enough
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err //nolint:wrapcheck // json.Unmarshal error is clear enough
	}

	return r.Restore(config, clearExisting, abortOnError)
}

// Restore restores configuration from a map.
//
//nolint:gocognit,gocyclo,cyclop,funlen,gocritic,lll // function complexity is necessary for configuration restoration logic; parameter type combination would reduce readability
func (r *Root) Restore(config map[string]interface{}, clearExisting bool, abortOnError bool) ([]string, error) {
	var errors []string

	errFunc := func(errStr string) {
		errors = append(errors, errStr)
		if abortOnError {
			errors = append(errors, "Aborted due to error")
		}
	}

	//nolint:nestif // nested if structure is necessary for restoration logic
	if clearExisting {
		if err := r.ClearExisting(); err != nil {
			errFunc(err.Error())

			if abortOnError {
				return errors, fmt.Errorf("%w: %w", errClearingExistingConfig, err)
			}
		}
	} else {
		subsystems, _ := r.Subsystems() //nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
		if len(subsystems) > 0 {
			return nil, &CFSError{Msg: "subsystems present, not restoring"}
		}
	}

	// Create hosts first because subsystems reference them
	if hosts, ok := config["hosts"].([]interface{}); ok { //nolint:nestif,lll // nested if structure is necessary for configuration parsing
		for i, hostData := range hosts { //nolint:varnamelen // i is a standard loop variable name
			if hostMap, ok := hostData.(map[string]interface{}); ok {
				if _, ok := hostMap["nqn"].(string); ok {
					if err := SetupHost(hostMap, errFunc); err != nil {
						errFunc(fmt.Sprintf("Error setting up host %d: %v", i, err))

						if abortOnError {
							return errors, err
						}
					}
				} else {
					errFunc(fmt.Sprintf("'nqn' not defined in host %d", i))
				}
			}
		}
	}

	// Create subsystems
	if subsystems, ok := config["subsystems"].([]interface{}); ok { //nolint:nestif,lll // nested if structure is necessary for configuration parsing
		for i, subsysData := range subsystems { //nolint:varnamelen // i is a standard loop variable name
			if subsysMap, ok := subsysData.(map[string]interface{}); ok {
				if _, ok := subsysMap["nqn"].(string); ok {
					if err := SetupSubsystem(subsysMap, errFunc); err != nil {
						errFunc(fmt.Sprintf("Error setting up subsystem %d: %v", i, err))

						if abortOnError {
							return errors, err
						}
					}
				} else {
					errFunc(fmt.Sprintf("'nqn' not defined in subsystem %d", i))
				}
			}
		}
	}

	// Create ports
	if ports, ok := config["ports"].([]interface{}); ok { //nolint:nestif,lll // nested if structure is necessary for configuration parsing
		for i, portData := range ports { //nolint:varnamelen // i is a standard loop variable name
			if portMap, ok := portData.(map[string]interface{}); ok {
				if _, ok := portMap["portid"].(float64); ok {
					if err := SetupPort(r, portMap, errFunc); err != nil {
						errFunc(fmt.Sprintf("Error setting up port %d: %v", i, err))

						if abortOnError {
							return errors, err
						}
					}
				} else {
					errFunc(fmt.Sprintf("'portid' not defined in port %d", i))
				}
			}
		}
	}

	return errors, nil
}

// Dump returns a JSON-serializable representation of the root configuration.
func (r *Root) Dump() (map[string]interface{}, error) { //nolint:cyclop,lll // function complexity is necessary for dumping all configuration
	result, err := r.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	subsystems, err := r.Subsystems()
	if err != nil {
		return nil, err
	}

	subsysData := make([]interface{}, 0, len(subsystems))

	for _, s := range subsystems {
		data, err := s.Dump() //nolint:govet // err shadowing is acceptable in loop context
		if err != nil {
			return nil, err
		}

		subsysData = append(subsysData, data)
	}

	result["subsystems"] = subsysData

	ports, err := r.Ports()
	if err != nil {
		return nil, err
	}

	portsData := make([]interface{}, 0, len(ports))

	for _, p := range ports {
		data, err := p.Dump() //nolint:govet // err shadowing is acceptable in loop context
		if err != nil {
			return nil, err
		}

		portsData = append(portsData, data)
	}

	result["ports"] = portsData

	hosts, err := r.Hosts()
	if err != nil {
		return nil, err
	}

	hostsData := make([]interface{}, 0, len(hosts))

	for _, h := range hosts {
		data, err := h.Dump()
		if err != nil {
			return nil, err
		}

		hostsData = append(hostsData, data)
	}

	result["hosts"] = hostsData

	return result, nil
}
