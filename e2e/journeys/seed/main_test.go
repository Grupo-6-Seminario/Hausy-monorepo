package main

import "testing"

func TestRecreatesOnlyATestDatabase(t *testing.T) {
	for uri, want := range map[string]string{
		"postgresql://hausy:hausy@localhost:5432/hausy_e2e_test": "postgresql://hausy:hausy@localhost:5432/postgres",
		"postgres://u:p@db:5433/x_test?sslmode=disable":          "postgres://u:p@db:5433/postgres?sslmode=disable",
		"postgresql://hausy:hausy@localhost:5432/hausy":          "",
		"postgresql://hausy:hausy@localhost:5432/hausy_test_dev": "",
	} {
		admin, _, err := adminURI(uri)
		if admin != want || (want == "") != (err != nil) {
			t.Errorf("adminURI(%q) = %q, %v; want %q", uri, admin, err, want)
		}
	}
}
