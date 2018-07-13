package tar

import (
	"fmt"
	"testing"

	"errors"
	r "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"io/ioutil"
	"os"
	"path/filepath"
	"archive/tar"
	"io"
	"strings"
	"bytes"
)

const templateDirRenderingConfig = `
data "tar_template" "configs_dir" {
  source_dir = "%s"
  vars = %s
}`

type testTemplate struct {
	template string
	want     string
}

func testTemplateDirWriteFiles(files map[string]testTemplate) (in, out string, err error) {
	in, err = ioutil.TempDir(os.TempDir(), "terraform_test")
	if err != nil {
		return
	}

	for name, file := range files {
		path := filepath.Join(in, name)

		err = os.MkdirAll(filepath.Dir(path), 0777)
		if err != nil {
			return
		}

		err = ioutil.WriteFile(path, []byte(file.template), 0777)
		if err != nil {
			return
		}
	}

	out = fmt.Sprintf("%s.out", in)
	return
}

func TestTemplateDirRendering(t *testing.T) {
	var cases = []struct {
		vars  string
		files map[string]testTemplate
	}{
		{
			files: map[string]testTemplate{
				"foo.txt":           {"${bar}", "bar"},
				"nested/monkey.txt": {"ooh-ooh-ooh-eee-eee", "ooh-ooh-ooh-eee-eee"},
				"maths.txt":         {"${1+2+3}", "6"},
			},
			vars: `{bar = "bar"}`,
		},
	}

	for _, tt := range cases {
		// Write the desired templates in a temporary directory.
		in, out, err := testTemplateDirWriteFiles(tt.files)
		if err != nil {
			t.Skipf("could not write templates to temporary directory: %s", err)
			continue
		}
		defer os.RemoveAll(in)

		// Run test case.
		r.UnitTest(t, r.TestCase{
			Providers: testProviders,
			Steps: []r.TestStep{
				{
					Config: fmt.Sprintf(templateDirRenderingConfig, in, tt.vars),
					Check: func(s *terraform.State) error {
						module := s.ModuleByPath([]string{"root"})
						resource := module.Resources["data.tar_template.configs_dir"]
						rendered := resource.Primary.Attributes["rendered"]

						tr := tar.NewReader(strings.NewReader(rendered))
						for {
							hdr, err := tr.Next()
							if err == io.EOF {
								break // End of archive
							}
							if err != nil {
								return err
							}
							buf := new(bytes.Buffer)
							buf.ReadFrom(tr)
							fileContent := buf.String()

							name := hdr.Name
							want := tt.files[name].want
							if want != fileContent {
								return fmt.Errorf("file:\n%s\nvars:\n%s\ngot:\n%s\nwant:\n%s\n", name, tt.vars, err, want)
							}
						}
						return nil
					},
				},
			},
			CheckDestroy: func(*terraform.State) error {
				if _, err := os.Stat(out); os.IsNotExist(err) {
					return nil
				}
				return errors.New("tar_template did not get destroyed")
			},
		})
	}
}