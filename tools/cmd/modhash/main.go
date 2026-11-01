// Command modhash prints the h1: hash that the go command and
// sum.golang.org record for a module version, computed under Go's
// module-zip rules (nested modules such as tools/, VCS directories, vendor
// directories and symbolic links are excluded).
//
//	modhash -module github.com/entur/go-calendar -version v0.1.0 -rev v0.1.0 REPO
//	modhash -module github.com/entur/go-calendar -version v0.1.0 DIR
//
// With -rev it hashes the committed tree at that git revision of the
// repository REPO, using `git archive` as the go command does when it
// fetches from version control; untracked files are ignored. Without -rev
// it hashes the files in DIR as they are.
//
// The release workflow compares the -rev output for the release tag with
// the hash `go mod download -json` reports through proxy.golang.org, which
// proves that the content recorded in the checksum database is the tagged
// tree.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	"golang.org/x/mod/zip"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "modhash:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("modhash", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("module", "", "module path (required)")
	version := fs.String("version", "", "module version, for example v0.1.0 (required)")
	rev := fs.String("rev", "", "git revision to hash instead of the files in DIR")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *path == "" || *version == "" || fs.NArg() != 1 {
		return errors.New("usage: modhash -module PATH -version VERSION [-rev REV] DIR")
	}
	h, err := hash(module.Version{Path: *path, Version: *version}, fs.Arg(0), *rev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, h)
	return err
}

// hash builds the module zip in a temporary file and returns its h1: hash.
func hash(m module.Version, dir, rev string) (_ string, err error) {
	tmp, err := os.CreateTemp("", "modhash-*.zip")
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, os.Remove(tmp.Name()))
	}()
	create := func() error { return zip.CreateFromDir(tmp, m, dir) }
	if rev != "" {
		create = func() error { return zip.CreateFromVCS(tmp, m, dir, rev, "") }
	}
	if err := create(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("create module zip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return dirhash.HashZip(tmp.Name(), dirhash.Hash1)
}
