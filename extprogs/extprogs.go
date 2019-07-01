package extprogs

import (
	"github.com/nsf/termbox-go"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"os"
	"os/exec"
)

// View shows given bytes in external text editor
func View(name string, content []byte) {
	termbox.Close()
	defer termbox.Init()

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogFatal("extprogs.View() ", err)
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
		logging.LogFatal("extprogs.View() ", err)
	}
}

// Edit shows given bytes in external text editor and returns changed contents
// of file if file is changed
func Edit(name string, content []byte) (changed bool, newContent []byte) {
	termbox.Close()
	defer termbox.Init()

	f, err := ioutil.TempFile(".", name)
	if err != nil {
		logging.LogFatal("extprogs.Edit() ", err)
	}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	fileInfo, err := os.Stat(f.Name())
	if err != nil {
		logging.LogFatal("extprogs.Edit() ", err)
	}
	time1 := fileInfo.ModTime()

	viewCmd := exec.Command(config.Conf.Cmd.Editor, f.Name())

	viewCmd.Stdout = os.Stdout
	viewCmd.Stderr = os.Stderr
	viewCmd.Stdin = os.Stdin

	err = viewCmd.Run()
	if err != nil {
		logging.LogFatal("extprogs.Edit() ", err)
	}

	fileInfo, err = os.Stat(f.Name())
	if err != nil {
		logging.LogFatal("extprogs.Edit() ", err)
	}
	time2 := fileInfo.ModTime()
	if time1 != time2 {
		newContent, err := ioutil.ReadFile(f.Name())
		if err != nil {
			logging.LogFatal("extprogs.Edit(): ", err)
		}
		return true, newContent
	}
	return false, nil
}
