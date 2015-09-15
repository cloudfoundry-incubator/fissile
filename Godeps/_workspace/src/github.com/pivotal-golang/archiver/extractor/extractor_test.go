package extractor_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-golang/archiver/extractor"
	"github.com/pivotal-golang/archiver/extractor/test_helper"
)

var _ = Describe("Extractor", func() {
	var extractor Extractor

	var extractionDest string
	var extractionSrc string

	BeforeEach(func() {
		var err error

		archive, err := ioutil.TempFile("", "extractor-archive")
		Ω(err).ShouldNot(HaveOccurred())

		extractionDest, err = ioutil.TempDir("", "extracted")
		Ω(err).ShouldNot(HaveOccurred())

		extractionSrc = archive.Name()

		extractor = NewDetectable()
	})

	AfterEach(func() {
		os.RemoveAll(extractionSrc)
		os.RemoveAll(extractionDest)
	})

	archiveFiles := []test_helper.ArchiveFile{
		{
			Name: "./",
			Dir:  true,
		},
		{
			Name: "./some-file",
			Body: "some-file-contents",
		},
		{
			Name: "./empty-dir/",
			Dir:  true,
		},
		{
			Name: "./nonempty-dir/",
			Dir:  true,
		},
		{
			Name: "./nonempty-dir/file-in-dir",
			Body: "file-in-dir-contents",
		},
		{
			Name: "./legit-exe-not-a-virus.bat",
			Mode: 0644,
			Body: "rm -rf /",
		},
		{
			Name: "./some-symlink",
			Link: "some-file",
			Mode: 0755,
		},
	}

	extractionTest := func() {
		err := extractor.Extract(extractionSrc, extractionDest)
		Ω(err).ShouldNot(HaveOccurred())

		fileContents, err := ioutil.ReadFile(filepath.Join(extractionDest, "some-file"))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(fileContents)).Should(Equal("some-file-contents"))

		fileContents, err = ioutil.ReadFile(filepath.Join(extractionDest, "nonempty-dir", "file-in-dir"))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(fileContents)).Should(Equal("file-in-dir-contents"))

		executable, err := os.Open(filepath.Join(extractionDest, "legit-exe-not-a-virus.bat"))
		Ω(err).ShouldNot(HaveOccurred())

		executableInfo, err := executable.Stat()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(executableInfo.Mode()).Should(Equal(os.FileMode(0644)))

		emptyDir, err := os.Open(filepath.Join(extractionDest, "empty-dir"))
		Ω(err).ShouldNot(HaveOccurred())

		emptyDirInfo, err := emptyDir.Stat()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(emptyDirInfo.IsDir()).Should(BeTrue())

		target, err := os.Readlink(filepath.Join(extractionDest, "some-symlink"))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(target).Should(Equal("some-file"))

		symlinkInfo, err := os.Lstat(filepath.Join(extractionDest, "some-symlink"))
		Ω(err).ShouldNot(HaveOccurred())

		Ω(symlinkInfo.Mode() & 0755).Should(Equal(os.FileMode(0755)))
	}

	Context("when the file is a zip archive", func() {
		BeforeEach(func() {
			test_helper.CreateZipArchive(extractionSrc, archiveFiles)
		})

		Context("when 'unzip' is on the PATH", func() {
			BeforeEach(func() {
				_, err := exec.LookPath("unzip")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("extracts the ZIP's files, generating directories, and honoring file permissions and symlinks", extractionTest)
		})

		Context("when 'unzip' is not in the PATH", func() {
			var oldPATH string

			BeforeEach(func() {
				oldPATH = os.Getenv("PATH")
				os.Setenv("PATH", "/dev/null")

				_, err := exec.LookPath("unzip")
				Ω(err).Should(HaveOccurred())
			})

			AfterEach(func() {
				os.Setenv("PATH", oldPATH)
			})

			It("extracts the ZIP's files, generating directories, and honoring file permissions and symlinks", extractionTest)
		})
	})

	Context("when the file is a tgz archive", func() {
		BeforeEach(func() {
			test_helper.CreateTarGZArchive(extractionSrc, archiveFiles)
		})

		Context("when 'tar' is on the PATH", func() {
			BeforeEach(func() {
				_, err := exec.LookPath("tar")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("extracts the TGZ's files, generating directories, and honoring file permissions and symlinks", extractionTest)
		})

		Context("when 'tar' is not in the PATH", func() {
			var oldPATH string

			BeforeEach(func() {
				oldPATH = os.Getenv("PATH")
				os.Setenv("PATH", "/dev/null")

				_, err := exec.LookPath("tar")
				Ω(err).Should(HaveOccurred())
			})

			AfterEach(func() {
				os.Setenv("PATH", oldPATH)
			})

			It("extracts the TGZ's files, generating directories, and honoring file permissions and symlinks", extractionTest)
		})
	})
})
