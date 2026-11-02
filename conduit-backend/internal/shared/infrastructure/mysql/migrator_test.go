package mysql

import (
	"reflect"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := map[string]struct {
		script string
		want   []string
	}{
		"empty":               {"", []string{}},
		"single":              {"CREATE TABLE a (id INT);", []string{"CREATE TABLE a (id INT)"}},
		"no trailing newline": {"SELECT 1", []string{"SELECT 1"}},
		"multiple":            {"SELECT 1;\n\nSELECT 2;\n", []string{"SELECT 1", "SELECT 2"}},
		"semicolon in string": {"INSERT INTO t VALUES ('a;b');SELECT 2", []string{"INSERT INTO t VALUES ('a;b')", "SELECT 2"}},
		"escaped quote":       {`INSERT INTO t VALUES ('it\'s; fine');SELECT 2`, []string{`INSERT INTO t VALUES ('it\'s; fine')`, "SELECT 2"}},
		"doubled quote":       {"INSERT INTO t VALUES ('it''s; fine');SELECT 2", []string{"INSERT INTO t VALUES ('it''s; fine')", "SELECT 2"}},
		"backtick identifier": {"SELECT `a;b` FROM t;SELECT 2", []string{"SELECT `a;b` FROM t", "SELECT 2"}},
		"blank statements":    {";;SELECT 1;;", []string{"SELECT 1"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := splitStatements(tc.script); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitStatements(%q) = %#v, want %#v", tc.script, got, tc.want)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	cfg, err := ParseURL("mysql://dev_user:p%40ss@localhost/conduit_dev?timeout=3s")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User != "dev_user" || cfg.DBName != "conduit_dev" || cfg.Addr != "localhost:3306" || cfg.Net != "tcp" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.ParseTime || !cfg.ClientFoundRows || cfg.Loc.String() != "UTC" {
		t.Fatalf("expected forced driver options, got %+v", cfg)
	}
	if cfg.Params["transaction_isolation"] != "'READ-COMMITTED'" {
		t.Fatalf("expected read committed default, got %v", cfg.Params)
	}
}

func TestParseURLAcceptsNativeDSN(t *testing.T) {
	cfg, err := ParseURL("u:p@tcp(db:3307)/app")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "db:3307" || cfg.DBName != "app" || !cfg.ParseTime {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseURLRejectsMissingHost(t *testing.T) {
	if _, err := ParseURL("mysql:///conduit"); err == nil {
		t.Fatal("expected an error for a missing host")
	}
}

func TestConstraintNameIgnoresOtherErrors(t *testing.T) {
	if ConstraintName(nil) != "" || IsUniqueViolation(nil) {
		t.Fatal("nil must not be a unique violation")
	}
}
