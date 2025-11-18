package nvmet_test

import (
	"os"
	"testing"

	"github.com/Djarvur/go-nvmetcli/internal/nvmet"
)

func TestRoot(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	subsystems, err := root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 0 {
		t.Errorf("Expected 0 subsystems after clear, got %d", len(subsystems))
	}

	ports, err := root.Ports()
	if err != nil {
		t.Fatalf("Failed to list ports: %v", err)
	}

	if len(ports) != 0 {
		t.Errorf("Expected 0 ports after clear, got %d", len(ports))
	}

	hosts, err := root.Hosts()
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}

	if len(hosts) != 0 {
		t.Errorf("Expected 0 hosts after clear, got %d", len(hosts))
	}
}

//nolint:cyclop // test function complexity is necessary for comprehensive testing
func TestSubsystem(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	// Create mode
	//nolint:varnamelen // s1 is a test variable name
	s1, err := nvmet.NewSubsystem("testnqn1", "create")
	if err != nil {
		t.Fatalf("Failed to create subsystem: %v", err)
	}

	if s1 == nil {
		t.Fatal("Subsystem is nil")
	}

	if s1.NQN() != "testnqn1" {
		t.Errorf("Expected NQN 'testnqn1', got '%s'", s1.NQN())
	}

	subsystems, err := root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 1 {
		t.Errorf("Expected 1 subsystem, got %d", len(subsystems))
	}

	// Any mode, should create
	_, err = nvmet.NewSubsystem("testnqn2", "any")
	if err != nil {
		t.Fatalf("Failed to create subsystem: %v", err)
	}

	subsystems, err = root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 2 {
		t.Errorf("Expected 2 subsystems, got %d", len(subsystems))
	}

	// Random name
	s3, err := nvmet.NewSubsystem("", "create")
	if err != nil {
		t.Fatalf("Failed to create subsystem with random name: %v", err)
	}

	if s3.NQN() == "" {
		t.Error("Subsystem NQN is empty")
	}

	subsystems, err = root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 3 {
		t.Errorf("Expected 3 subsystems, got %d", len(subsystems))
	}

	// Duplicate
	_, err = nvmet.NewSubsystem("testnqn1", "create")
	if err == nil {
		t.Error("Expected error when creating duplicate subsystem")
	}

	// Lookup using any, should not create
	//nolint:varnamelen // s is a test variable name
	s, err := nvmet.NewSubsystem("testnqn1", "any")
	if err != nil {
		t.Fatalf("Failed to lookup subsystem: %v", err)
	}

	if s.NQN() != s1.NQN() {
		t.Errorf("Expected NQN '%s', got '%s'", s1.NQN(), s.NQN())
	}

	// Lookup only
	s, err = nvmet.NewSubsystem("testnqn2", "lookup")
	if err != nil {
		t.Fatalf("Failed to lookup subsystem: %v", err)
	}

	if s.NQN() != "testnqn2" {
		t.Errorf("Expected NQN 'testnqn2', got '%s'", s.NQN())
	}

	// And delete them all
	for _, s := range subsystems {
		//nolint:govet // err shadowing is acceptable in test context
		if err := s.Delete(); err != nil {
			t.Errorf("Failed to delete subsystem: %v", err)
		}
	}

	subsystems, err = root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 0 {
		t.Errorf("Expected 0 subsystems after delete, got %d", len(subsystems))
	}
}

//nolint:cyclop // test function complexity is necessary for comprehensive testing
func TestNamespace(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	//nolint:varnamelen // s is a test variable name
	s, err := nvmet.NewSubsystem("testnqn", "create")
	if err != nil {
		t.Fatalf("Failed to create subsystem: %v", err)
	}

	namespaces, err := s.Namespaces()
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}

	if len(namespaces) != 0 {
		t.Errorf("Expected 0 namespaces in new subsystem, got %d", len(namespaces))
	}

	// Create mode
	//nolint:varnamelen // n1 is a test variable name
	n1, err := nvmet.NewNamespace(s, 3, "create")
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	if n1 == nil {
		t.Fatal("Namespace is nil")
	}

	if n1.NSID() != 3 {
		t.Errorf("Expected NSID 3, got %d", n1.NSID())
	}

	namespaces, err = s.Namespaces()
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}

	if len(namespaces) != 1 {
		t.Errorf("Expected 1 namespace, got %d", len(namespaces))
	}

	// Any mode, should create
	_, err = nvmet.NewNamespace(s, 2, "any")
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	namespaces, err = s.Namespaces()
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}

	if len(namespaces) != 2 {
		t.Errorf("Expected 2 namespaces, got %d", len(namespaces))
	}

	// Create without nsid, should pick lowest available
	//nolint:varnamelen // n3 is a test variable name
	n3, err := nvmet.NewNamespace(s, 0, "create")
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	if n3.NSID() != 1 {
		t.Errorf("Expected NSID 1, got %d", n3.NSID())
	}

	namespaces, err = s.Namespaces()
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}

	if len(namespaces) != 3 {
		t.Errorf("Expected 3 namespaces, got %d", len(namespaces))
	}

	// And delete them all
	for _, ns := range namespaces {
		//nolint:govet // err shadowing is acceptable in test context
		if err := ns.Delete(); err != nil {
			t.Errorf("Failed to delete namespace: %v", err)
		}
	}

	namespaces, err = s.Namespaces()
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}

	if len(namespaces) != 0 {
		t.Errorf("Expected 0 namespaces after delete, got %d", len(namespaces))
	}

	// Cleanup
	if err := s.Delete(); err != nil {
		t.Errorf("Failed to delete subsystem: %v", err)
	}
}

//nolint:dupl,cyclop // test structure is similar but tests different functionality; test complexity is necessary for comprehensive testing
func TestPort(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	// Create mode
	//nolint:varnamelen // p1 is a test variable name
	p1, err := nvmet.NewPort(1, "create")
	if err != nil {
		t.Fatalf("Failed to create port: %v", err)
	}

	if p1 == nil {
		t.Fatal("Port is nil")
	}

	if p1.PortID() != 1 {
		t.Errorf("Expected PortID 1, got %d", p1.PortID())
	}

	ports, err := root.Ports()
	if err != nil {
		t.Fatalf("Failed to list ports: %v", err)
	}

	if len(ports) != 1 {
		t.Errorf("Expected 1 port, got %d", len(ports))
	}

	// Any mode, should create
	_, err = nvmet.NewPort(2, "any")
	if err != nil {
		t.Fatalf("Failed to create port: %v", err)
	}

	ports, err = root.Ports()
	if err != nil {
		t.Fatalf("Failed to list ports: %v", err)
	}

	if len(ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(ports))
	}

	// Duplicate
	_, err = nvmet.NewPort(1, "create")
	if err == nil {
		t.Error("Expected error when creating duplicate port")
	}

	// Lookup using any, should not create
	//nolint:varnamelen // p is a test variable name
	p, err := nvmet.NewPort(1, "any")
	if err != nil {
		t.Fatalf("Failed to lookup port: %v", err)
	}

	if p.PortID() != p1.PortID() {
		t.Errorf("Expected PortID %d, got %d", p1.PortID(), p.PortID())
	}

	// And delete them all
	for _, p := range ports {
		//nolint:govet // err shadowing is acceptable in test context
		if err := p.Delete(); err != nil {
			t.Errorf("Failed to delete port: %v", err)
		}
	}

	ports, err = root.Ports()
	if err != nil {
		t.Fatalf("Failed to list ports: %v", err)
	}

	if len(ports) != 0 {
		t.Errorf("Expected 0 ports after delete, got %d", len(ports))
	}
}

//nolint:dupl,cyclop // test structure is similar but tests different functionality; test complexity is necessary for comprehensive testing
func TestHost(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	// Create mode
	//nolint:varnamelen // h1 is a test variable name
	h1, err := nvmet.NewHost("hostnqn1", "create")
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}

	if h1 == nil {
		t.Fatal("Host is nil")
	}

	if h1.NQN() != "hostnqn1" {
		t.Errorf("Expected NQN 'hostnqn1', got '%s'", h1.NQN())
	}

	hosts, err := root.Hosts()
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}

	if len(hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(hosts))
	}

	// Any mode, should create
	_, err = nvmet.NewHost("hostnqn2", "any")
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}

	hosts, err = root.Hosts()
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}

	if len(hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(hosts))
	}

	// Duplicate
	_, err = nvmet.NewHost("hostnqn1", "create")
	if err == nil {
		t.Error("Expected error when creating duplicate host")
	}

	// Lookup using any, should not create
	//nolint:varnamelen // h is a test variable name
	h, err := nvmet.NewHost("hostnqn1", "any")
	if err != nil {
		t.Fatalf("Failed to lookup host: %v", err)
	}

	if h.NQN() != h1.NQN() {
		t.Errorf("Expected NQN '%s', got '%s'", h1.NQN(), h.NQN())
	}

	// And delete them all
	for _, h := range hosts {
		//nolint:govet // err shadowing is acceptable in test context
		if err := h.Delete(); err != nil {
			t.Errorf("Failed to delete host: %v", err)
		}
	}

	hosts, err = root.Hosts()
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}

	if len(hosts) != 0 {
		t.Errorf("Expected 0 hosts after delete, got %d", len(hosts))
	}
}

//nolint:cyclop // test function complexity is necessary for comprehensive testing
func TestSaveRestore(t *testing.T) {
	root, err := nvmet.NewRoot()
	if err != nil {
		t.Skipf("Skipping test: cannot access configfs: %v (requires root)", err)

		return
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	// Create some configuration
	_, err = nvmet.NewHost("hostnqn", "create")
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}

	//nolint:varnamelen // s is a test variable name
	s, err := nvmet.NewSubsystem("testnqn", "create")
	if err != nil {
		t.Fatalf("Failed to create subsystem: %v", err)
	}

	//nolint:govet // err shadowing is acceptable in test context
	if err := s.AddAllowedHost("hostnqn"); err != nil {
		t.Fatalf("Failed to add allowed host: %v", err)
	}

	_, err = nvmet.NewNamespace(s, 42, "create")
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	// Save configuration
	savefile := "/tmp/nvmetcli_test.json"
	//nolint:govet // err shadowing is acceptable in test context
	if err := root.SaveToFile(savefile); err != nil {
		t.Fatalf("Failed to save configuration: %v", err)
	}

	// Clear and restore
	//nolint:govet // err shadowing is acceptable in test context
	if err := root.ClearExisting(); err != nil {
		t.Fatalf("Failed to clear existing config: %v", err)
	}

	errors, err := root.RestoreFromFile(savefile, true, false)
	if err != nil {
		t.Fatalf("Failed to restore configuration: %v", err)
	}

	if len(errors) > 0 {
		for _, e := range errors {
			t.Logf("Restore warning: %s", e)
		}
	}

	// Verify restoration
	hosts, err := root.Hosts()
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}

	if len(hosts) != 1 {
		t.Errorf("Expected 1 host after restore, got %d", len(hosts))
	}

	subsystems, err := root.Subsystems()
	if err != nil {
		t.Fatalf("Failed to list subsystems: %v", err)
	}

	if len(subsystems) != 1 {
		t.Errorf("Expected 1 subsystem after restore, got %d", len(subsystems))
	}

	if len(subsystems) > 0 {
		namespaces, err := subsystems[0].Namespaces()
		if err != nil {
			t.Fatalf("Failed to list namespaces: %v", err)
		}

		if len(namespaces) != 1 {
			t.Errorf("Expected 1 namespace after restore, got %d", len(namespaces))
		}

		if len(namespaces) > 0 && namespaces[0].NSID() != 42 {
			t.Errorf("Expected NSID 42 after restore, got %d", namespaces[0].NSID())
		}
	}

	// Cleanup
	_ = root.ClearExisting()
	_ = os.Remove(savefile)
}
