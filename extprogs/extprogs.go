package extprogs

import (
	"errors"
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/nsf/termbox-go"
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
	logging.LogDebugf("extprogs.View('%s', '%s...')", name, string(content[:debugContentLen]))
	if config.Conf.Cmd.Viewer == "" {
		return errors.New("Viewer command not configured.")
	}

	termbox.Close()
	defer termbox.Init()

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogDebug("extprogs.View() ", err)
		return err
	}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	viewCmd := exec.Command(config.Conf.Cmd.Viewer, f.Name())

	viewCmd.Stdout = os.Stdout
	viewCmd.Stderr = os.Stderr
	viewCmd.Stdin = os.Stdin

	err = viewCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs.View() ", err)
	}

	return err
}

// Edit shows given bytes in external text editor and returns changed contents
// of file if file is changed
func Edit(name string, content []byte) (returnError error, changed bool, newContent []byte) {
	logging.LogDebug("extprogs.Edit() ", name)
	if config.Conf.Cmd.Editor == "" {
		return errors.New("Editor command not configured."), false, nil
	}

	termbox.Close()
	defer termbox.Init()

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogDebug("extprogs.Edit() ", err)
		return err, false, nil
	}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	fileInfo, err := os.Stat(f.Name())
	if err != nil {
		logging.LogDebug("extprogs.Edit() ", err)
		return err, false, nil
	}
	time1 := fileInfo.ModTime()

	viewCmd := exec.Command(config.Conf.Cmd.Editor, f.Name())

	viewCmd.Stdout = os.Stdout
	viewCmd.Stderr = os.Stderr
	viewCmd.Stdin = os.Stdin

	err = viewCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs.Edit() ", err)
		return err, false, nil
	}

	fileInfo, err = os.Stat(f.Name())
	if err != nil {
		logging.LogDebug("extprogs.Edit() ", err)
		return err, false, nil
	}
	time2 := fileInfo.ModTime()
	if time1 != time2 {
		newContent, err := ioutil.ReadFile(f.Name())
		if err != nil {
			logging.LogDebug("extprogs.Edit(): ", err)
			return err, false, nil
		}
		return nil, true, newContent
	}

	return nil, false, nil
}

// CreateTempDir creates temporary directory required for Diff()
func CreateTempDir(dirPrefix string) string {
	dir, err := ioutil.TempDir(".", dirPrefix)
	if err != nil {
		logging.LogFatal("extprogs.CreateTempDir() ", err)
	}
	return dir
}

// DeleteTempDir deletes temporary directory created for Diff()
func DeleteTempDir(dirPath string) string {
	err := os.RemoveAll(dirPath)
	if err != nil {
		logging.LogDebug("extprogs.DeleteTempDir() ", err)
		return fmt.Sprintf("Error deleting '%s' dir.", dirPath)
	}
	return ""
}

// Diff compares files/directories in external diff
func Diff(leftPath string, rightPath string) error {
	logging.LogDebug("extprogs.Diff() ", leftPath, " - ", rightPath)
	if config.Conf.Cmd.Diff == "" {
		return errors.New("Diff command not configured.")
	}

	termbox.Close()
	defer termbox.Init()
	diffCmd := exec.Command(config.Conf.Cmd.Diff, leftPath, rightPath)

	diffCmd.Stdout = os.Stdout
	diffCmd.Stderr = os.Stderr
	diffCmd.Stdin = os.Stdin

	err := diffCmd.Run()
	if err != nil {
		logging.LogDebug("extprogs.Diff() err: ", err)
	}

	return err
}
