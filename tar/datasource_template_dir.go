package tar

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/helper/pathorcontents"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultFileMode = 0600

type templateRenderError error

func dataSourceDir() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDirRead,

		Schema: map[string]*schema.Schema{
			"source_dir": {
				Type:        schema.TypeString,
				Optional:    false,
				Required:    true,
				Description: "Path to the directory where the files to template reside",
			},
			"vars": {
				Type:         schema.TypeMap,
				Optional:     true,
				Default:      make(map[string]interface{}),
				Description:  "Variables to substitute",
				ValidateFunc: validateVarsAttribute,
				ForceNew:     true,
			},
			"rendered": &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "rendered template",
			},
		},
	}
}

func dataSourceDirRead(d *schema.ResourceData, meta interface{}) error {
	rendered, err := renderDir(d)
	if err != nil {
		return err
	}
	d.Set("rendered", rendered)
	d.SetId(hashDir(rendered))
	return nil
}

func renderDir(d *schema.ResourceData) (string, error) {
	vars := d.Get("vars").(map[string]interface{})
	rendered := ""
	if dir, ok := d.GetOk("source_dir"); ok {
		tarData, err := tarDirAsString(dir.(string), vars)
		if err != nil {
			return "", fmt.Errorf("could not generate output checksum: %s", err)
		}
		rendered = tarData
	}

	return rendered, nil
}

func hashDir(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
}

func tarDirAsString(directoryPath string, vars map[string]interface{}) (string, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	writeFile := func(p string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(directoryPath, p)
		if relPath == "." {
			return nil
		}

		var header *tar.Header
		header, err = tar.FileInfoHeader(f, f.Name())
		if err != nil {
			return err
		}

		zeroTime := time.Unix(0, 0)
		header.ChangeTime = zeroTime
		header.AccessTime = zeroTime
		header.ModTime = zeroTime
		header.Uname = ""
		header.Gname = ""
		header.Uid = 0
		header.Gid = 0

		if f.IsDir() {
			header.Name = relPath
			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			return nil
		} else {
			inputContent, _, err := pathorcontents.Read(p)
			if err != nil {
				return err
			}

			outputContent, err := execute(inputContent, vars)
			if err != nil {
				return templateRenderError(fmt.Errorf("failed to render %v: %v", p, err))
			}

			header := &tar.Header{
				Name: relPath,
				Mode: DefaultFileMode,
				Size: int64(len(outputContent)),
			}

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if _, err := tw.Write([]byte(outputContent)); err != nil {
				log.Fatal(err)
			}
		}

		return err
	}

	if err := filepath.Walk(directoryPath, writeFile); err != nil {
		return "", err
	}
	if err := tw.Flush(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// execute parses and executes a template using vars.
func execute(s string, vars map[string]interface{}) (string, error) {
	root, err := hil.Parse(s)
	if err != nil {
		return "", err
	}

	varmap := make(map[string]ast.Variable)
	for k, v := range vars {
		// As far as I can tell, v is always a string.
		// If it's not, tell the user gracefully.
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type for variable %q: %T", k, v)
		}
		varmap[k] = ast.Variable{
			Value: s,
			Type:  ast.TypeString,
		}
	}

	cfg := hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap:  varmap,
			FuncMap: config.Funcs(),
		},
	}

	result, err := hil.Eval(root, &cfg)
	if err != nil {
		return "", err
	}
	if result.Type != hil.TypeString {
		return "", fmt.Errorf("unexpected output hil.Type: %v", result.Type)
	}

	return result.Value.(string), nil
}

func validateVarsAttribute(v interface{}, key string) (ws []string, es []error) {
	// vars can only be primitives right now
	var badVars []string
	for k, v := range v.(map[string]interface{}) {
		switch v.(type) {
		case []interface{}:
			badVars = append(badVars, fmt.Sprintf("%s (list)", k))
		case map[string]interface{}:
			badVars = append(badVars, fmt.Sprintf("%s (map)", k))
		}
	}
	if len(badVars) > 0 {
		es = append(es, fmt.Errorf(
			"%s: cannot contain non-primitives; bad keys: %s",
			key, strings.Join(badVars, ", ")))
	}
	return
}
