package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

var (
	baseDir   = getPWD()
	terraFile = filepath.Join(baseDir, `Terratar.yml`)
	modPath   = filepath.Dir(terraFile) + `/.vendor/modules`
	overwrite bool
)

func main() {
	pf := pflag.NewFlagSet(`terratar`, pflag.ExitOnError)
	pf.StringVarP(&terraFile, "terratar-file", "f", terraFile, "Filepath of Terratar file.")
	pf.StringVarP(&modPath, "module-path", "p", modPath, "Modules install directory, Defaults to directory of Terratar config file.")
	pf.BoolVar(&overwrite, "overwrite", false, "Overwrite if module directory already exists, otherwise ignore.")
	pf.Parse(os.Args[1:])
	terraFile = getAbs(terraFile)
	if !pf.Changed("module-path") {
		modPath = filepath.Dir(terraFile) + `/.vendor/modules`
	}
	modPath = getAbs(modPath)
	if !FileExists(terraFile) {
		log.Fatalf("[Terratar] specified config does not exist: %s\n", terraFile)
	}
	file, err := ioutil.ReadFile(terraFile)
	if err != nil {
		log.Fatalf("[Terratar] error reading %s: %v\n", terraFile, err)
	}
	var TT []source
	err = yaml.Unmarshal(file, &TT)
	if err != nil {
		log.Fatalf("[Terratar] error processing %s: %v\n", terraFile, err)
	}
	for _, src := range TT {
		tarFile := src.Version
		target := modPath + `/` + src.Name
		switch {
		case overwrite:
			log.Printf("[Terratar] Overriding existing Module at %s\n", target)
		case FileExists(target):
			log.Printf("[Terratar] Module already exists at %s, skipping...\n", target)
			continue
		}
		switch {
		case src.Source == "":
			log.Fatalf("[Terratar] invalid source for %s\n", src.Name)
		case tarFile == "":
			log.Fatalf("[Terratar] invalid version for %s\n", src.Name)
		}
		tarFile = strings.TrimSuffix(tarFile, `.tar.gz`)
		tarFile = tarFile + `.tar.gz`
		URL, _ := url.Parse(src.Source)
		URL.Path = path.Join(URL.Path, tarFile)
		log.Printf("[Terratar] Downloading: %s\n", URL.String())
		resp, err := http.Get(URL.String())
		if err != nil {
			log.Printf("[Terratar] error retrieving tarball from %s: %v\n", target, err)
			continue
		}
		defer resp.Body.Close()
		err = Untar(target, resp.Body)
		if err != nil {
			log.Printf("[Terratar] error processing tarball from %s: %v\n", target, err)
			continue
		}
	}
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		if err == io.EOF {
			err = fmt.Errorf("[Terratar] tarball not found or empty")
		}
		log.Printf("[Terratar] invalid tarball for module: %s\n", dst)
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			log.Printf("[Terratar] Saved Module Files to %s\n", dst)
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}
		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			f.Close()
		}
	}
}

func getPWD() string {
	d, err := os.Getwd()
	if err != nil {
		log.Printf("[Terratar] Error retrieving working directory: %v\n", err)
	}
	return d
}

func getAbs(path string) string {
	d, err := filepath.Abs(path)
	if err != nil {
		log.Printf("[Terratar] Error validating %s: %v\n", path, err)
	}
	return d
}

// FileExists checks for the existence of the file indicated by filename and returns true if it exists.
func FileExists(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

type source struct {
	Name    string `yaml:"name" json:"name"`
	Source  string `yaml:"source" json:"source"`
	Version string `yaml:"version" json:"version"`
}
