package nvmet

import (
	"fmt"
	"path/filepath"
)

// Host represents an NVMe Host in configFS.
//
//nolint:govet // field alignment optimization is not critical for this struct
type Host struct {
	CFSNode
	nqn string
}

// NewHost creates a new Host instance.
//
//nolint:gocritic // parameter type combination would reduce readability
func NewHost(nqn string, mode string) (*Host, error) {
	//nolint:exhaustruct,varnamelen // CFSNode and nqn are initialized below; h is a standard short variable name for host
	h := &Host{}
	h.CFSNode.path = configfsDir
	h.attrGroups = []string{}

	if nqn == "" {
		return nil, &CFSError{Msg: "NQN cannot be empty for Host"}
	}

	h.nqn = nqn
	h.path = filepath.Join(configfsDir, "hosts", nqn)

	if err := h.createInCFS(mode); err != nil {
		return nil, err
	}

	return h, nil
}

// NQN returns the NQN of the host.
func (h *Host) NQN() string {
	return h.nqn
}

// SetupHost sets up a Host based on a map from saved config.
func SetupHost(data map[string]interface{}, errFunc func(string)) error {
	nqn, ok := data["nqn"].(string)
	if !ok {
		errFunc("'nqn' not defined for Host")

		return nil
	}

	_, err := NewHost(nqn, "any")
	if err != nil {
		errFunc(fmt.Sprintf("Could not create Host object: %v", err))

		return nil
	}

	return nil
}

// Dump returns a JSON-serializable representation of the host.
func (h *Host) Dump() (map[string]interface{}, error) {
	result, err := h.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	result["nqn"] = h.nqn

	return result, nil
}
