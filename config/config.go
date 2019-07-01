package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/howeyc/gopass"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
)

const (
	configDirName  = ".dpcmder"
	configFileName = "config.json"
)

// LocalFolderPath is a folder where dpcmder start showing files - set by command flag.
var LocalFolderPath *string

// DebugLogFile enables writing of debug messages to dpcmder.log file in current folder.
var DebugLogFile *bool

// DataPower configuration from command flags.
var (
	DpRestURL  *string
	DpSomaURL  *string
	DpUsername *string
	DpPassword *string
	DpDomain   *string
	Proxy      *string
)

type Config struct {
	Cmd  Command
	Log  Log
	Sync Sync
}

type Command struct {
	Viewer string
	Editor string
}

type Log struct {
	MaxEntrySize int
}

type Sync struct {
	Seconds int
}

//var Cmd = Command{Viewer: "less", Editor: "vi"}
var Conf = Config{Cmd: Command{Viewer: "less", Editor: "vi"}, Log: Log{MaxEntrySize: logging.MaxEntrySize}, Sync: Sync{Seconds: 4}}

// ParseProgramArgs parses program arguments and fill config package variables with flag values.
func parseProgramArgs() {
	LocalFolderPath = flag.String("l", "", "")
	DpRestURL = flag.String("r", "", "")
	DpSomaURL = flag.String("s", "", "")
	DpUsername = flag.String("u", "", "")
	DpPassword = flag.String("p", "", "")
	DpDomain = flag.String("d", "", "")
	Proxy = flag.String("x", "", "")
	DebugLogFile = flag.Bool("debug", false, "write dpcmder.log file")

	flag.Parse()
}

// Init intializes configuration: parses command line flags and creates config directory.
func Init() {
	parseProgramArgs()
	logging.DebugLogFile = *DebugLogFile
	logging.LogDebug("dpcmder starting...")
	validateProgramArgs()
	initConfigDir()
	readConfig()
}

// ValidateProgramArgs parsed program arguments and asks for password input and/or
// shows usage message in case some mandatory arguments are missing.
func validateProgramArgs() {
	if *LocalFolderPath == "" ||
		(*DpUsername != "" && *DpRestURL == "" && *DpSomaURL == "") {
		usage()
	}

	if *DpUsername != "" {
		if *DpPassword == "" {
			fmt.Println("DataPower password: ")
			// Silent. For printing *'s use gopass.GetPasswdMasked()
			pass, err := gopass.GetPasswdMasked()
			if err != nil {
				usage()
			} else {
				*DpPassword = string(pass)
			}
		}

		if *DpPassword == "" {
			fmt.Println("Password can't be empty!")
			fmt.Println()
			usage()
		}
	}
}

func configDirPath() string {
	usr, err := user.Current()
	if err != nil {
		logging.LogFatal("Can't find current user: ", err)
	}

	println("usr.HomeDir: ", usr.HomeDir)
	configDirPath := utils.GetFilePath(usr.HomeDir, configDirName)

	return configDirPath
}

func configFilePath() string {
	configDirPath := configDirPath()

	_, err := os.Stat(configDirPath)
	if err != nil {
		err = os.Mkdir(configDirPath, os.ModePerm)
		if err != nil {
			logging.LogFatal("Can't create configuration directory: ", err)
		}
	}

	configFilePath := utils.GetFilePath(configDirPath, configFileName)

	return configFilePath
}

func initConfigDir() {
	configFilePath := configFilePath()
	_, err := os.Stat(configFilePath)
	if err != nil {
		file, err := os.Create(configFilePath)
		if err != nil {
			logging.LogFatal("Can't create configuration file: ", err)
		}
		defer file.Close()

		configBytes, err := json.MarshalIndent(Conf, "", "  ")
		if err != nil {
			logging.LogFatal("Can't marshall configuration object: ", err)
		}

		_, err = file.Write(configBytes)
		if err != nil {
			logging.LogFatal("Can't write configuration file: ", err)
		}
	}
}

func readConfig() {
	logging.LogDebug("Conf before read: ", Conf)
	configFilePath := configFilePath()

	configFileBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		logging.LogFatal("Can't read configuration file: ", err)
	}

	err = json.Unmarshal(configFileBytes, &Conf)
	if err != nil {
		logging.LogFatal("Can't unmarshal configuration file: ", err)
	}

	logging.MaxEntrySize = Conf.Log.MaxEntrySize
	logging.LogDebug("Conf after read:  ", Conf)
}

// DpUseRest returns true if we configured dpcmder to use DataPower REST Management interface.
func DpUseRest() bool {
	return *DpRestURL != ""
}

// DpUseSoma returns true if we configured dpcmder to use DataPower SOMA Management interface.
func DpUseSoma() bool {
	return *DpSomaURL != ""
}

// PrintConfig prints configuration values to console.
func PrintConfig() {
	fmt.Println("LocalFolderPath: ", *LocalFolderPath)
	fmt.Println("DpRestURL: ", *DpRestURL)
	fmt.Println("DpSomaURL: ", *DpSomaURL)
	fmt.Println("DpUsername: ", *DpUsername)
	fmt.Println("DpPassword: ", strings.Repeat("*", len(*DpPassword)))
	fmt.Println("DpDomain: ", *DpDomain)
	fmt.Println("Proxy: ", *Proxy)
}

func usage() {
	fmt.Println("Usage:")
	fmt.Printf(" %s -l LOCAL_FOLDER_PATH [-r DATA_POWER_REST_URL | -s DATA_POWER_SOMA_AMP_URL] [-u USERNAME] [-p PASSWORD] [-d DP_DOMAIN] [-x PROXY_SERVER] [-debug]\n", os.Args[0])
	fmt.Println("")
	fmt.Println(" -debug flag turns on creation of dpcmder.log file with debug log messages")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Printf(" %s -l .\n", os.Args[0])
	fmt.Println("   - will run only local file browser without connection to DataPower")
	fmt.Printf(" %s -l . -r https://172.17.0.2:5554 -u admin\n", os.Args[0])
	fmt.Printf(" %s -l . -s https://172.17.0.2:5550 -u admin -p admin -d default -debug\n", os.Args[0])

	os.Exit(1)
}
