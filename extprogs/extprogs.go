// Package extprogs contains code for calling external application used for
// file viewing, edditing and comparing. File viewer is also used to display
// help inside dpcmder application.
package extprogs

import (
	"bytes"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/help"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

// View shows given bytes in external text editor.
func View(name string, content []byte) error {
	return viewBytes(name, content, true)
}

// viewBytes shows given bytes in external text editor - with optional
// terminal exit/init.
func viewBytes(name string, content []byte, consoleActive bool) error {
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

	return viewFile(f.Name(), consoleActive)
}

// ViewFile shows file from given path in external viewer.
func ViewFile(filePath string) error {
	return viewFile(filePath, true)
}

// viewFile shows file from given path in external viewer - with optional
// terminal exit/init.
func viewFile(filePath string, consoleActive bool) error {
	logging.LogDebugf("extprogs/ViewFile('%s')", filePath)
	if config.Conf.Cmd.Viewer == "" {
		return errs.Error("Viewer command not configured - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.")
	}

	if consoleActive {
		out.Stop()
		defer out.Init()
	}

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
// of file if file is changed.
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
// of file if file is changed.
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

// CreateTempDir creates temporary directory required for Diff().
func CreateTempDir(dirPrefix string) string {
	dir, err := ioutil.TempDir(".", dirPrefix)
	logging.LogDebugf("extprogs/CreateTempDir('%s') dir: '%s'", dirPrefix, dir)
	if err != nil {
		logging.LogDebugf("extprogs/CreateTempDir('%s') - err: %v", dirPrefix, err)
	}
	return dir
}

// DeleteTempDir deletes temporary directory created for Diff().
func DeleteTempDir(dirPath string) error {
	logging.LogDebugf("extprogs/DeleteTempDir('%s') ", dirPath)
	err := os.RemoveAll(dirPath)
	if err != nil {
		logging.LogDebug("extprogs/DeleteTempDir() ", err)
		return errs.Errorf("Error deleting '%s' dir.", dirPath)
	}
	return nil
}

// Diff compares files/directories in external diff.
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

	timeStart := time.Now().UnixNano()
	err := diffCmd.Run()
	dt := time.Duration(time.Now().UnixNano() - timeStart)

	// If error is quickly returned it could be non-blocking diff was used for
	// Diff command - in that case repeat command but show it's output in viewer
	if err != nil && err.Error() == "exit status 1" && dt < 100*time.Millisecond {
		logging.LogDebugf(
			"extprogs/Diff() '%s' command returns error '%s' in %d, will try to re-execute Diff comand and send result to View command.",
			config.Conf.Cmd.Diff, err, dt)

		diffCmd = exec.Command(config.Conf.Cmd.Diff, leftPath, rightPath)
		var buf bytes.Buffer
		diffCmd.Stdout = &buf
		diffCmd.Stderr = &buf
		err = diffCmd.Run()
		logging.LogDebugf("extprogs/Diff() err: %v", err)
		err = viewBytes("Diff result", buf.Bytes(), false)

		if err != nil {
			logging.LogDebug("extprogs/Diff() err: ", err)
			return errs.Errorf("Diff command misconfigured: '%s' - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.", err)
		}

		return errs.Errorf("Diff command misconfigured, to use non-blocking diff: '%s' - check ~/.dpcmder/config.json and/or run dpcmder with -help flag.",
			config.Conf.Cmd.Diff)
	}

	return nil
}

// ShowHelp shows help in configured external viewer.
func ShowHelp() error {
	return View("Help", []byte(help.Help))
}
