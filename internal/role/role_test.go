package role

import "testing"

func TestGetValidRoles(t *testing.T) {
	for _, name := range ValidRoles() {
		p := Get(name)
		if p.Name != name {
			t.Errorf("Get(%q).Name = %q", name, p.Name)
		}
		if p.Description == "" {
			t.Errorf("Get(%q).Description is empty", name)
		}
		if p.Permission == "" {
			t.Errorf("Get(%q).Permission is empty", name)
		}
		if p.SystemHint == "" {
			t.Errorf("Get(%q).SystemHint is empty", name)
		}
	}
}

func TestGetDefaultsToDev(t *testing.T) {
	p := Get("nonexistent")
	if p.Name != "dev" {
		t.Errorf("expected dev fallback, got %q", p.Name)
	}
}

func TestValidRolesCount(t *testing.T) {
	roles := ValidRoles()
	if len(roles) != 5 {
		t.Errorf("expected 5 roles, got %d", len(roles))
	}
}

func TestRolePermissions(t *testing.T) {
	expected := map[string]string{
		"dev":   "workspace_write",
		"test":  "readonly",
		"ops":   "full_access",
		"sense": "readonly",
		"scout": "readonly",
	}
	for name, perm := range expected {
		p := Get(name)
		if p.Permission != perm {
			t.Errorf("Get(%q).Permission = %q, want %q", name, p.Permission, perm)
		}
	}
}
