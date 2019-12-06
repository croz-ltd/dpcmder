package extprogs

import (
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/help"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"os"
	"os/exec"
)

// View shows given bytes in external text editor
func View(name string, content []byte) error {
	debugContentLen := len(content)
	if debugContentLen > 20 {
		debugContentLen = 20
	}
	logging.LogDebugf("extprogs/View('%s', '%s...')", name, string(content[:debugContentLen]))
	if config.Conf.Cmd.Viewer == "" {
		return errs.Error("Viewer command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogDebug("extprogs/View() ", err)
		return err
	}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	return ViewFile(f.Name())
}

// ViewFile shows file from given path in external viewer
func ViewFile(filePath string) error {
	logging.LogDebugf("extprogs/ViewFile('%s')", filePath)
	if config.Conf.Cmd.Viewer == "" {
		return errs.Error("Viewer command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	out.Stop()
	defer out.Init()

	viewCmd := exec.Command(config.Conf.Cmd.Viewer, filePath)

	viewCmd.Stdout = os.Stdout
	viewCmd.Stderr = os.Stderr
	viewCmd.Stdin = os.Stdin

	err := viewCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs/ViewFile() ", err)
		return errs.Errorf("Viewer command misconfigured: '%s' - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.", err)
	}

	return nil
}

// Edit shows given bytes in external text editor and returns changed contents
// of file if file is changed
func Edit(name string, content []byte) (changed bool, newContent []byte, returnError error) {
	logging.LogDebugf("extprogs/Edit('%s', ..) ", name)
	if config.Conf.Cmd.Editor == "" {
		return false, nil, errs.Error("Editor command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogDebug("extprogs/Edit() ", err)
		return false, nil, err
	}

	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	fileInfo, err := os.Stat(f.Name())
	if err != nil {
		logging.LogDebug("extprogs/Edit() ", err)
		return false, nil, err
	}
	time1 := fileInfo.ModTime()

	err = EditFile(f.Name())
	if err != nil {
		logging.LogDebug("extprogs/Edit() ", err)
		return false, nil, err
	}

	fileInfo, err = os.Stat(f.Name())
	if err != nil {
		logging.LogDebug("extprogs/Edit() ", err)
		return false, nil, err
	}
	time2 := fileInfo.ModTime()
	if time1 != time2 {
		newContent, err := ioutil.ReadFile(f.Name())
		if err != nil {
			logging.LogDebug("extprogs/Edit(): ", err)
			return false, nil, nil
		}
		return true, newContent, nil
	}

	return false, nil, nil
}

// EditFile shows given file external text editor and returns changed contents
// of file if file is changed
func EditFile(filePath string) error {
	logging.LogDebugf("extprogs/EditFile('%s') ", filePath)
	if config.Conf.Cmd.Editor == "" {
		return errs.Error("Editor command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	out.Stop()
	defer out.Init()

	editCmd := exec.Command(config.Conf.Cmd.Editor, filePath)

	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	editCmd.Stdin = os.Stdin

	err := editCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs/EditFile() ", err)
		return errs.Errorf("Editor command misconfigured: '%s' - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.", err)
	}

	return nil
}

// CreateTempDir creates temporary directory required for Diff()
func CreateTempDir(dirPrefix string) string {
	dir, err := ioutil.TempDir(".", dirPrefix)
	logging.LogDebugf("extprogs/CreateTempDir('%s') dir: '%s'", dirPrefix, dir)
	if err != nil {
		logging.LogDebugf("extprogs/CreateTempDir('%s') - err: %v", dirPrefix, err)
	}
	return dir
}

// DeleteTempDir deletes temporary directory created for Diff()
func DeleteTempDir(dirPath string) error {
	logging.LogDebugf("extprogs/DeleteTempDir('%s') ", dirPath)
	err := os.RemoveAll(dirPath)
	if err != nil {
		logging.LogDebug("extprogs/DeleteTempDir() ", err)
		return errs.Errorf("Error deleting '%s' dir.", dirPath)
	}
	return nil
}

// Diff compares files/directories in external diff
func Diff(leftPath string, rightPath string) error {
	logging.LogDebugf("extprogs/Diff('%s', '%s')", leftPath, rightPath)
	if config.Conf.Cmd.Diff == "" {
		return errs.Error("Diff command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	out.Stop()
	defer out.Init()

	diffCmd := exec.Command(config.Conf.Cmd.Diff, leftPath, rightPath)

	diffCmd.Stdout = os.Stdout
	diffCmd.Stderr = os.Stderr
	diffCmd.Stdin = os.Stdin

	err := diffCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs/Diff() err: ", err)
		return errs.Errorf("Diff command misconfigured: '%s' - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.", err)
	}

	return nil
}

// ShowHelp shows help in configured external viewer.
func ShowHelp() error {
	return View("Help", []byte(help.Help))
}
