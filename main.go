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
	terraFile, modPath string
)

func main() {
	pf := pflag.NewFlagSet(`terratar`, pflag.ExitOnError)
	pf.StringVarP(&terraFile, "terratar-file", "f", "./Terratar.yml", "Filepath of Terratar file.")
	pf.StringVarP(&modPath, "module-path", "p", "./vendor/modules", "Filepath where generated modules will install.")
	pf.Parse(os.Args[1:])
	file, err := ioutil.ReadFile(terraFile)
	if err != nil {
		log.Fatalf("error reading %s: %v\n", terraFile, err)
	}
	var TT map[string]map[string]string
	err = yaml.Unmarshal(file, &TT)
	if err != nil {
		log.Fatalf("error processing %s: %v\n", terraFile, err)
	}
	for modName, tarParams := range TT {
		var useURL, tarFile string
		var ok bool
		if useURL, ok = tarParams[`source`]; !ok {
			log.Fatalf("invalid source for %s\n", modName)
		}
		if tarFile, ok = tarParams[`version`]; !ok {
			log.Fatalf("invalid version for %s\n", modName)
		}
		tarFile = strings.TrimSuffix(tarFile, `.tar.gz`)
		tarFile = tarFile + `.tar.gz`
		URL, _ := url.Parse(useURL)
		URL.Path = path.Join(URL.Path, tarFile)
		log.Printf("[Terratar] Downloading: %s\n", URL.String())
		resp, err := http.Get(URL.String())
		if err != nil {
			log.Printf("error retrieving tarball: %v\n", err)
			return
		}
		defer resp.Body.Close()
		target := modPath + `/` + modName
		err = Untar(target, resp.Body)
		if err != nil {
			log.Printf("error processing tarball: %v\n", err)
			return
		}
	}
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		if err == io.EOF {
			err = fmt.Errorf("tarball not found or empty")
		}
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
