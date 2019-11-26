package config

import (
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/croz-ltd/confident"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"github.com/howeyc/gopass"
	"os"
	"os/user"
	"strings"
)

const (
	configDirName  = ".dpcmder"
	configFileName = "config.json"
	// PreviousApplianceName is name of configuration for the last appliance
	// configured with command-line parameters (without explicitly saving config).
	PreviousApplianceName = "_PreviousAppliance_"
)

// CurrentAppliance stores configuration value of current appliance used.
var CurrentAppliance DataPowerAppliance

// CurrentApplianceName stores configuration of current appliance name used.
var CurrentApplianceName string

// LocalFolderPath is a folder where dpcmder start showing files - set by command flag.
var LocalFolderPath *string

// DebugLogFile/TraceLogFile enables writing of debug/trace messages to
// dpcmder.log file in current folder.
var (
	DebugLogFile *bool
	TraceLogFile *bool
)

// DataPower configuration from command flags.
var (
	dpRestURL    *string
	dpSomaURL    *string
	dpUsername   *string
	dpPassword   *string
	dpDomain     *string
	proxy        *string
	dpConfigName *string
	help         *bool
)

// DpTransientPasswordMap contains passwords entered through dpcmder usage,
// not saved to config during [other] configuration changes.
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
	Diff   string
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

func (dpa *DataPowerAppliance) SetDpPlaintextPassword(password string) {
	b32password := base32.StdEncoding.EncodeToString([]byte(password))
	dpa.Password = b32password
}
func (dpa *DataPowerAppliance) DpPlaintextPassword() string {
	passBytes, err := base32.StdEncoding.DecodeString(dpa.Password)
	if err != nil {
		logging.LogFatal("config/DataPowerAppliance.DpPlaintextPassword() - Can't decode password: ", err)
	}
	return string(passBytes)
}

//var Cmd = Command{Viewer: "less", Editor: "vi"}
var Conf = Config{
	Cmd: Command{
		Viewer: "less", Editor: "vi", Diff: "ldiff"},
	Log:                 Log{MaxEntrySize: logging.MaxEntrySize},
	Sync:                Sync{Seconds: 4},
	DataPowerAppliances: make(map[string]DataPowerAppliance)}

var k *confident.Confident

func confidentBootstrap() {
	k = confident.New()
	k.WithConfiguration(&Conf)
	// <Optional>
	k.Name = "config"
	k.Type = "json"
	k.Path = configDirPath()
	k.Path = configDirPathEnsureExists()
	k.Permission = os.FileMode(0644)
	// </Optional>
	logging.LogDebug("config/confidentBootstrap() - Conf before read: ", Conf)
	k.Read()
	logging.LogDebug("config/confidentBootstrap() - Conf after read: ", Conf)
	if *dpRestURL != "" || *dpSomaURL != "" {
		if *dpConfigName != "" {
			Conf.DataPowerAppliances[*dpConfigName] = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
			CurrentApplianceName = *dpConfigName
		} else {
			Conf.DataPowerAppliances[PreviousApplianceName] = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
			CurrentApplianceName = PreviousApplianceName
		}
		k.Persist()
	}
	CurrentAppliance = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
	logging.LogDebug("config/confidentBootstrap() - Conf after persist: ", Conf)
}

// ParseProgramArgs parses program arguments and fill config package variables with flag values.
func parseProgramArgs() {
	LocalFolderPath = flag.String("l", ".", "Path to local directory to open, default is '.'")
	dpRestURL = flag.String("r", "", "DataPower REST URL")
	dpSomaURL = flag.String("s", "", "DataPower SOMA URL")
	dpUsername = flag.String("u", "", "DataPower user username")
	password := flag.String("p", "", "DataPower user password")
	dpDomain = flag.String("d", "", "DataPower domain name")
	proxy = flag.String("x", "", "URL of proxy server for DataPower connection")
	dpConfigName = flag.String("c", "", "Name of DataPower connection configuration to save with given configuration params")
	DebugLogFile = flag.Bool("debug", false, "Write debug dpcmder.log file in current dir")
	TraceLogFile = flag.Bool("trace", false, "Write trace dpcmder.log file in current dir")
	help = flag.Bool("h", false, "Show dpcmder usage with examples")

	flag.Parse()
	setDpPasswordPlain(*password)
}

// Init intializes configuration: parses command line flags and creates config directory.
func Init() {
	parseProgramArgs()
	logging.DebugLogFile = *DebugLogFile
	logging.TraceLogFile = *TraceLogFile
	logging.LogDebug("config/Init() - dpcmder starting...")
	validateProgramArgs()
	confidentBootstrap()
	validatePassword()
}

// validateProgramArgs validate parsed program arguments and/or shows usage
// message in case some mandatory arguments are missing.
func validateProgramArgs() {
	if *help {
		usage(0)
	}

	if *LocalFolderPath == "" ||
		(*dpUsername != "" && *dpRestURL == "" && *dpSomaURL == "") {
		usage(1)
	}
}

// validatePassword validates password argument and asks for password input and/or
// shows usage message in case it is missing.
func validatePassword() {
	if *dpUsername != "" {
		if *dpPassword == "" {
			fmt.Println("DataPower password: ")
			// Silent. For printing *'s use gopass.GetPasswdMasked()
			pass, err := gopass.GetPasswdMasked()
			if err != nil {
				usage(1)
			} else {
				password := string(pass)
				setDpPasswordPlain(password)
			}
		}

		if *dpPassword == "" {
			fmt.Println("Password can't be empty!")
			fmt.Println()
			usage(1)
		}
	}
}

func configDirPath() string {
	usr, err := user.Current()
	if err != nil {
		logging.LogFatal("config/configDirPath() - Can't find current user: ", err)
	}

	// println("usr.HomeDir: ", usr.HomeDir)
	configDirPath := paths.GetFilePath(usr.HomeDir, configDirName)

	return configDirPath
}

func configDirPathEnsureExists() string {
	configDirPath := configDirPath()

	_, err := os.Stat(configDirPath)
	if err != nil {
		err = os.Mkdir(configDirPath, os.ModePerm)
		if err != nil {
			logging.LogFatal("config/configDirPathEnsureExists() - Can't create configuration directory: ", err)
		}
	}

	return configDirPath
}

func setDpPasswordPlain(password string) {
	b32password := base32.StdEncoding.EncodeToString([]byte(password))
	dpPassword = &b32password
}

// GetDpApplianceConfig fetches DataPower appliance JSON configuration as byte array.
func (c *Config) GetDpApplianceConfig(name string) ([]byte, error) {
	dpAppliance := c.DataPowerAppliances[name]
	json, err := json.MarshalIndent(dpAppliance, "", "  ")
	if err != nil {
		logging.LogDebugf("config/GetDpApplianceConfig('%s') - Can't marshal DataPower appliance configuration: ", name)
		return nil, err
	}
	return json, nil
}

// SetDpApplianceConfig sets DataPower appliance JSON configuration as byte array.
func (c *Config) SetDpApplianceConfig(name string, contents []byte) error {
	dpAppliance := c.DataPowerAppliances[name]
	err := json.Unmarshal(contents, &dpAppliance)
	if err != nil {
		logging.LogDebugf("config/SetDpApplianceConfig('%s', ...) - Can't unmarshal DataPower appliance configuration: ", name)
		return err
	}
	c.DataPowerAppliances[name] = dpAppliance
	k.Persist()
	return nil
}

// PrintConfig prints configuration values to console.
func PrintConfig() {
	fmt.Println("LocalFolderPath: ", *LocalFolderPath)
	fmt.Println("dpRestURL: ", *dpRestURL)
	fmt.Println("dpSomaURL: ", *dpSomaURL)
	fmt.Println("dpUsername: ", *dpUsername)
	fmt.Println("DpPassword: ", strings.Repeat("*", len(*dpPassword)))
	fmt.Println("dpDomain: ", *dpDomain)
	fmt.Println("proxy: ", *proxy)
	fmt.Println("dpConfigName: ", *dpConfigName)
	fmt.Println("help: ", *help)
}

// LogConfig logs configuration values to log file.
func LogConfig() {
	logging.LogDebug("LocalFolderPath: ", *LocalFolderPath)
	logging.LogDebug("dpRestURL: ", *dpRestURL)
	logging.LogDebug("dpSomaURL: ", *dpSomaURL)
	logging.LogDebug("dpUsername: ", *dpUsername)
	logging.LogDebug("DpPassword: ", strings.Repeat("*", len(*dpPassword)))
	logging.LogDebug("dpDomain: ", *dpDomain)
	logging.LogDebug("proxy: ", *proxy)
	logging.LogDebug("dpConfigName: ", *dpConfigName)
	logging.LogDebug("help: ", *help)
}

// usage prints usage help information with examples to console.
func usage(exitStatus int) {
	fmt.Println("Usage:")
	fmt.Printf(" %s [-l LOCAL_FOLDER_PATH] [-r DATA_POWER_REST_URL | -s DATA_POWER_SOMA_AMP_URL] [-u USERNAME] [-p PASSWORD] [-d DP_DOMAIN] [-x PROXY_SERVER] [-c DP_CONFIG_NAME] [-debug] [-h]\n", os.Args[0])
	fmt.Println("")
	fmt.Println(" -l LOCAL_FOLDER_PATH - set path to local folder")
	fmt.Println(" -r DATA_POWER_REST_URL - set REST management URL for DataPower")
	fmt.Println(" -s DATA_POWER_SOMA_AMP_URL - set SOMA URL for DataPower")
	fmt.Println(" -u USERNAME - set username to connect to DataPower")
	fmt.Println(" -p PASSWORD - set password to connect to DataPower")
	fmt.Println(" -d DP_DOMAIN - connect to specific DataPower domain (can be neccessary on some security configurations)")
	fmt.Println(" -x PROXY_SERVER - connect to DataPower through proxy")
	fmt.Println(" -c DP_CONFIG_NAME - save DataPower configuration under given name")
	fmt.Println(" -debug - turns on creation of dpcmder.log file with debug log messages")
	fmt.Println(" -h - shows this help")
	fmt.Println("")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Printf(" %s\n", os.Args[0])
	fmt.Println("   - will run local file browser (in current dir) with available DataPower connection configurations shown")
	fmt.Printf(" %s -l . -r https://172.17.0.2:5554 -u admin\n", os.Args[0])
	fmt.Printf(" %s -l . -s https://172.17.0.2:5550 -u admin -p admin -d default -debug\n", os.Args[0])
	fmt.Printf(" %s -l . -s https://172.17.0.2:5550 -u admin -p admin -d default -c LocalDp\n", os.Args[0])

	os.Exit(exitStatus)
}
