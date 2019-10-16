package config

import (
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/croz-ltd/confident"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/howeyc/gopass"
	"os"
	"os/user"
	"strings"
)

const (
	configDirName     = ".dpcmder"
	configFileName    = "config.json"
	PreviousAppliance = "_PreviousAppliance_"
)

// LocalFolderPath is a folder where dpcmder start showing files - set by command flag.
var LocalFolderPath *string

// DebugLogFile enables writing of debug messages to dpcmder.log file in current folder.
var DebugLogFile *bool

// Help flag shows dpcmder usage help.
var Help *bool

// DataPower configuration from command flags.
var (
	DpRestURL      *string
	DpSomaURL      *string
	DpUsername     *string
	dpPassword     *string
	DpDomain       *string
	Proxy          *string
	DpTemplateName *string
)

// DpTransientPasswordMap contains passwords entered through dpcmder usage and not saved to config.
var DpTransientPasswordMap = make(map[string]string)

type Config struct {
	Cmd                 Command
	Log                 Log
	Sync                Sync
	DataPowerAppliances map[string]DataPowerAppliance
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

type DataPowerAppliance struct {
	RestUrl  string
	SomaUrl  string
	Username string
	Password string
	Domain   string
	Proxy    string
}

//var Cmd = Command{Viewer: "less", Editor: "vi"}
var Conf = Config{Cmd: Command{Viewer: "less", Editor: "vi"}, Log: Log{MaxEntrySize: logging.MaxEntrySize}, Sync: Sync{Seconds: 4}, DataPowerAppliances: make(map[string]DataPowerAppliance)}

var k *confident.Confident

func confidentBootstrap() {
	k = confident.New()
	k.WithConfiguration(&Conf)
	// <Optional>
	k.Name = "config"
	k.Type = "json"
	k.Path = configDirPathEnsureExists()
	k.Permission = os.FileMode(0644)
	// </Optional>
	logging.LogDebug("Conf before read: ", Conf)
	k.Read()
	logging.LogDebug("Conf after read: ", Conf)
	if *DpRestURL != "" || *DpSomaURL != "" {
		if *DpTemplateName != "" {
			Conf.DataPowerAppliances[*DpTemplateName] = DataPowerAppliance{Domain: *DpDomain, Proxy: *Proxy, RestUrl: *DpRestURL, SomaUrl: *DpSomaURL, Username: *DpUsername, Password: *dpPassword}
		} else {
			Conf.DataPowerAppliances[PreviousAppliance] = DataPowerAppliance{Domain: *DpDomain, Proxy: *Proxy, RestUrl: *DpRestURL, SomaUrl: *DpSomaURL, Username: *DpUsername, Password: *dpPassword}
		}
		k.Persist()
		logging.LogDebug("Conf after persist: ", Conf)
	}
}

// ParseProgramArgs parses program arguments and fill config package variables with flag values.
func parseProgramArgs() {
	LocalFolderPath = flag.String("l", ".", "Path to local directory to open, default is '.'")
	DpRestURL = flag.String("r", "", "DataPower REST URL")
	DpSomaURL = flag.String("s", "", "DataPower SOMA URL")
	DpUsername = flag.String("u", "", "DataPower user username")
	password := flag.String("p", "", "DataPower user password")
	DpDomain = flag.String("d", "", "DataPower domain name")
	Proxy = flag.String("x", "", "URL of proxy server for DataPower connection")
	DpTemplateName = flag.String("t", "", "Name of DataPower connection config template to save with given configuration params")
	DebugLogFile = flag.Bool("debug", false, "Write dpcmder.log file in current dir")
	Help = flag.Bool("h", false, "Show dpcmder usage with examples")

	flag.Parse()
	SetDpPassword(*password)
}

// Init intializes configuration: parses command line flags and creates config directory.
func Init() {
	parseProgramArgs()
	logging.DebugLogFile = *DebugLogFile
	logging.LogDebug("dpcmder starting...")
	validateProgramArgs()
	confidentBootstrap()
}

func ClearDpConfig() {
	*DpRestURL = ""
	*DpSomaURL = ""
	*DpUsername = ""
	*dpPassword = ""
	*DpDomain = ""
	*Proxy = ""
}
func LoadDpConfig(configName string) {
	appliance := Conf.DataPowerAppliances[configName]

	*DpRestURL = appliance.RestUrl
	*DpSomaURL = appliance.SomaUrl
	*DpUsername = appliance.Username
	if appliance.Password != "" {
		*dpPassword = appliance.Password
	}
	*DpDomain = appliance.Domain
	*Proxy = appliance.Proxy
}

// ValidateProgramArgs parsed program arguments and asks for password input and/or
// shows usage message in case some mandatory arguments are missing.
func validateProgramArgs() {
	if *Help {
		usage()
	}

	if *LocalFolderPath == "" ||
		(*DpUsername != "" && *DpRestURL == "" && *DpSomaURL == "") {
		usage()
	}

	if *DpUsername != "" {
		if *dpPassword == "" {
			fmt.Println("DataPower password: ")
			// Silent. For printing *'s use gopass.GetPasswdMasked()
			pass, err := gopass.GetPasswdMasked()
			if err != nil {
				usage()
			} else {
				password := string(pass)
				SetDpPassword(password)
				SetDpTransientPassword(password)
			}
		}

		if *dpPassword == "" {
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

func configDirPathEnsureExists() string {
	configDirPath := configDirPath()

	_, err := os.Stat(configDirPath)
	if err != nil {
		err = os.Mkdir(configDirPath, os.ModePerm)
		if err != nil {
			logging.LogFatal("Can't create configuration directory: ", err)
		}
	}

	return configDirPath
}

// DpUseRest returns true if we configured dpcmder to use DataPower REST Management interface.
func DpUseRest() bool {
	return *DpRestURL != ""
}

// DpUseSoma returns true if we configured dpcmder to use DataPower SOMA Management interface.
func DpUseSoma() bool {
	return *DpSomaURL != ""
}

func SetDpPassword(password string) {
	b32password := base32.StdEncoding.EncodeToString([]byte(password))
	dpPassword = &b32password
}
func DpPassword() string {
	passBytes, err := base32.StdEncoding.DecodeString(*dpPassword)
	if err != nil {
		logging.LogFatal("Can't decode password: ", err)
	}
	return string(passBytes)
}

func SetDpTransientPassword(password string) {
	DpTransientPasswordMap[PreviousAppliance] = password
}

func (c *Config) GetDpApplianceConfig(name string) []byte {
	dpAppliance := c.DataPowerAppliances[name]
	json, err := json.MarshalIndent(dpAppliance, "", "  ")
	if err != nil {
		logging.LogFatal("Can't unmarshal DataPower appliance configuration: ", err)
	}
	return json
}

// PrintConfig prints configuration values to console.
func PrintConfig() {
	fmt.Println("LocalFolderPath: ", *LocalFolderPath)
	fmt.Println("DpRestURL: ", *DpRestURL)
	fmt.Println("DpSomaURL: ", *DpSomaURL)
	fmt.Println("DpUsername: ", *DpUsername)
	fmt.Println("DpPassword: ", strings.Repeat("*", len(*dpPassword)))
	fmt.Println("DpDomain: ", *DpDomain)
	fmt.Println("Proxy: ", *Proxy)
	fmt.Println("DpTemplateName: ", *DpTemplateName)
	fmt.Println("Help: ", *Help)
}

func usage() {
	fmt.Println("Usage:")
	fmt.Printf(" %s -l LOCAL_FOLDER_PATH [-r DATA_POWER_REST_URL | -s DATA_POWER_SOMA_AMP_URL] [-u USERNAME] [-p PASSWORD] [-d DP_DOMAIN] [-x PROXY_SERVER] [-t DP_TEMPLATE_NAME] [-debug]\n", os.Args[0])
	fmt.Println("")
	fmt.Println(" -l LOCAL_FOLDER_PATH - set path to local folder")
	fmt.Println(" -r DATA_POWER_REST_URL - set REST management URL for DataPower")
	fmt.Println(" -s DATA_POWER_SOMA_AMP_URL - set SOMA URL for DataPower")
	fmt.Println(" -u USERNAME - set username to connect to DataPower")
	fmt.Println(" -p PASSWORD - set password to connect to DataPower")
	fmt.Println(" -d DP_DOMAIN - connect to specific DataPower domain (can me neccessary on some security configurations)")
	fmt.Println(" -x PROXY_SERVER - connect to DataPower through proxy")
	fmt.Println(" -t DP_TEMPLATE_NAME - save DataPower configuration under given name")
	fmt.Println(" -debug flag turns on creation of dpcmder.log file with debug log messages")
	fmt.Println(" -h flag shows this help")
	fmt.Println("")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Printf(" %s\n", os.Args[0])
	fmt.Println("   - will run local file browser (in current dir) with available DataPower connection templates shown")
	fmt.Printf(" %s -l . -r https://172.17.0.2:5554 -u admin\n", os.Args[0])
	fmt.Printf(" %s -l . -s https://172.17.0.2:5550 -u admin -p admin -d default -debug\n", os.Args[0])
	fmt.Printf(" %s -l . -s https://172.17.0.2:5550 -u admin -p admin -d default -t LocalDp\n", os.Args[0])

	os.Exit(1)
}
