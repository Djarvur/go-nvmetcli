package nvmet

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// Namespace represents an NVMe Namespace in configFS.
//
//nolint:govet // field alignment optimization is not critical for this struct
type Namespace struct {
	CFSNode
	subsystem *Subsystem
	nsid      int
}

// NewNamespace creates a new Namespace instance.
//
//nolint:cyclop // function complexity is necessary for NSID auto-assignment logic
func NewNamespace(subsystem *Subsystem, nsid int, mode string) (*Namespace, error) {
	//nolint:exhaustruct,varnamelen,lll // CFSNode, subsystem, and nsid are initialized below; n is a standard short variable name for namespace
	n := &Namespace{}
	n.CFSNode.path = configfsDir
	n.attrGroups = []string{"device"}

	if subsystem == nil {
		return nil, &CFSError{Msg: "Invalid parent subsystem"}
	}

	if nsid == 0 {
		//nolint:goconst // "lookup" is a mode string, not a constant
		if mode == "lookup" {
			return nil, &CFSError{Msg: "Need NSID for lookup"}
		}

		// Find next available NSID
		//nolint:errcheck // ignoring error is acceptable here, we'll handle empty list
		existingNamespaces, _ := subsystem.Namespaces()

		existingNSIDs := make(map[int]bool)
		for _, ns := range existingNamespaces {
			existingNSIDs[ns.nsid] = true
		}

		for i := 1; i <= maxNSID; i++ {
			if !existingNSIDs[i] {
				nsid = i

				break
			}
		}

		if nsid == 0 {
			return nil, &CFSError{Msg: fmt.Sprintf("All NSIDs 1-%d in use", maxNSID)}
		}
	} else if nsid < 1 || nsid > maxNSID {
		return nil, &CFSError{Msg: fmt.Sprintf("NSID must be 1 to %d", maxNSID)}
	}

	n.subsystem = subsystem
	n.nsid = nsid
	n.path = filepath.Join(subsystem.path, "namespaces", strconv.Itoa(nsid))

	if err := n.createInCFS(mode); err != nil {
		return nil, err
	}

	return n, nil
}

// Subsystem returns the parent subsystem.
func (n *Namespace) Subsystem() *Subsystem {
	return n.subsystem
}

// NSID returns the namespace ID.
func (n *Namespace) NSID() int {
	return n.nsid
}

// SetupNamespace sets up a Namespace based on a map from saved config.
func SetupNamespace(subsys *Subsystem, data map[string]interface{}, errFunc func(string)) error {
	var (
		nsid int
		//nolint:varnamelen // ok is a standard Go idiom for type assertion result
		ok bool
	)

	switch v := data["nsid"].(type) {
	case int:
		nsid = v
		ok = true
	case float64:
		nsid = int(v)
		ok = true
	}

	if !ok {
		errFunc("'nsid' not defined for Namespace")

		return nil
	}

	//nolint:varnamelen // ns is a common abbreviation for namespace in this context
	ns, err := NewNamespace(subsys, nsid, "any")
	if err != nil {
		errFunc(fmt.Sprintf("Could not create Namespace object: %v", err))

		return nil
	}

	ns.setupAttrs(data, errFunc)

	return nil
}

// Dump returns a JSON-serializable representation of the namespace.
func (n *Namespace) Dump() (map[string]interface{}, error) {
	result, err := n.CFSNode.Dump()
	if err != nil {
		return nil, err
	}

	result["nsid"] = n.nsid

	return result, nil
}
