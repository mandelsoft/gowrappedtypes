package gowt_test

import (
	"os"
	"path/filepath"

	"github.com/mandelsoft/composer/epi"
	"github.com/mandelsoft/composer/filesystem"
	. "github.com/mandelsoft/composer/testenv"
	"github.com/mandelsoft/vfs/pkg/vfs"

	. "github.com/mandelsoft/gowrappedtypes/gowt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// newEnv creates a filesystem test environment without needing to pass the
// factory explicitly at every call site.
func newEnv(opts ...epi.Option) *filesystem.Environment {
	env, err := New(filesystem.New, opts...)
	Expect(err).NotTo(HaveOccurred())
	return env
}

// goldenTest runs the generator on testdata/<caseName>/directives.go, compares
// the output against a <type>_gowt.golden file in the same directory, and
// (when UPDATE_GOLDEN=1) regenerates that golden file from the actual output.
var _ = DescribeTable("gowt golden",
	func(caseName string) {
		env := newEnv(TestData())
		defer env.Cleanup()

		fs := env.GetFilesystem()
		dirPath := "/testdata/" + caseName + "/directives.go"

		results, err := GenerateGoFile(fs, dirPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(HaveLen(1))

		got := results[0].Content

		// Derive the golden file name from the generated file name.
		// results[0].Path is e.g. /testdata/basic/concrete_gowt.go
		resultBase := filepath.Base(results[0].Path)
		goldenBase := resultBase[:len(resultBase)-len(".go")] + ".golden"
		goldenVFSPath := "/testdata/" + caseName + "/" + goldenBase

		if os.Getenv("UPDATE_GOLDEN") == "1" {
			Expect(os.MkdirAll(filepath.Join("testdata", caseName), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join("testdata", caseName, goldenBase), got, 0644)).To(Succeed())
			return
		}

		want, err := vfs.ReadFile(fs, goldenVFSPath)
		Expect(err).NotTo(HaveOccurred(), "golden file %s not found; run with UPDATE_GOLDEN=1 to create it", goldenVFSPath)
		Expect(string(got)).To(Equal(string(want)))
	},
	Entry("basic — non-pointer base type, camelCase prefix", "basic"),
	Entry("pointer — pointer base type", "pointer"),
	Entry("renaming — explicit renaming clause", "renaming"),
	Entry("using — PACKAGE. placeholder with using clause", "using"),
	Entry("fixedfunc — non-generic function with TmplName references", "fixedfunc"),
	Entry("allfeatures — NEW+PACKAGE. in generic and non-generic functions", "allfeatures"),
	Entry("defaultvia — via omitted, defaults to BasePkg.New", "defaultvia"),
	Entry("unqualifiedvia — via with bare function name, uses base package", "unqualifiedvia"),
	Entry("suffix — prefix and suffix name substitution", "suffix"),
)

var _ = Describe("gowt error cases", func() {
	var fs vfs.FileSystem

	BeforeEach(func() {
		env := newEnv(TestData())
		DeferCleanup(env.Cleanup)
		fs = env.GetFilesystem()
	})

	It("rejects PACKAGE.NEW — contradictory via/using packages", func() {
		_, err := GenerateGoFile(fs, "/testdata/invalid_package_new/directives.go")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("PACKAGE.NEW is not valid"))
	})
})
