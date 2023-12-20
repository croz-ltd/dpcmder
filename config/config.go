// Package config contains logic for reading dpcmder configuration from
// commandline parameters, reading and writting configuration to dpcmder
// JSON configuration file and showing command line usage / help.
package config

import (
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/croz-ltd/confident"
	"github.com/croz-ltd/dpcmder/help"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"golang.org/x/term"
)

const (
	// configDirName & configFileName are used to save / find dpcmder configuration.
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

// DataPower Commander command line parameters.
var (
	// LocalFolderPath is a folder where dpcmder starts showing files - set by command flag.
	LocalFolderPath *string
	// DebugLogFile/TraceLogFile enables writing of debug/trace messages to
	// dpcmder.log file in current folder.
	DebugLogFile *bool
	TraceLogFile *bool
	// DataPower connection parameters.
	dpRestURL    *string
	dpSomaURL    *string
	dpUsername   *string
	dpPassword   *string
	dpDomain     *string
	proxy        *string
	dpConfigName *string
	// Help/Usage/Version flags - shows usage, help or version and exit.
	helpUsage *bool
	helpFull  *bool
	version   *bool
)

// DpTransientPasswordMap contains passwords entered through dpcmder dialogs,
// not saved to config during (other) configuration changes.
var DpTransientPasswordMap = make(map[string]string)

// Config is a structure containing dpcmder configuration (saved to JSON).
type Config struct {
	Cmd                 Command
	Log                 Log
	Sync                Sync
	DataPowerAppliances map[string]DataPowerAppliance
}

// Command is a structure containing dpcmder external command configuration.
type Command struct {
	Viewer string
	Editor string
	Diff   string
}

// Log is a structure containing dpcmder logging configuration. MaxEntrySize
// configures how many bytes can each log line contain (rest is removed).
type Log struct {
	MaxEntrySize int
}

// Sync is a structure containing dpcmder synchronization configuration used
// when syncing local filesystem to datapower is enabled.
type Sync struct {
	Seconds int
}

// DataPowerAppliance is a structure containing dpcmder DataPower appliance
// configuration details required to connect to appliances.
type DataPowerAppliance struct {
	RestUrl  string
	SomaUrl  string
	Username string
	Password string
	Domain   string
	Proxy    string
}

// List of DataPower management interfaces - returned by DpManagmentInterface().
const (
	DpInterfaceSoma    = "SOMA"
	DpInterfaceRest    = "REST"
	DpInterfaceUnknown = "Unknown"
)

// SetDpPlaintextPassword sets encoded Password field on DataPowerAppliance
// from plaintext password.
func (dpa *DataPowerAppliance) SetDpPlaintextPassword(password string) {
	b32password := base32.StdEncoding.EncodeToString([]byte(password))
	dpa.Password = b32password
}

// DpPlaintextPassword fetches decoded password from DataPowerAppliance struct.
func (dpa *DataPowerAppliance) DpPlaintextPassword() string {
	passBytes, err := base32.StdEncoding.DecodeString(dpa.Password)
	if err != nil {
		logging.LogDebugf("config/DataPowerAppliance.DpPlaintextPassword() - Can't decode password: '%s', err: %v",
			dpa.Password, err)
		return ""
	}
	return string(passBytes)
}

// DpManagmentInterface returns management interface used to manage DataPower.
func (dpa *DataPowerAppliance) DpManagmentInterface() string {
	switch {
	case dpa.RestUrl != "":
		return DpInterfaceRest
	case dpa.SomaUrl != "":
		return DpInterfaceSoma
	default:
		return DpInterfaceUnknown
	}
}

// Conf variable contains all configuration parameters for dpcmder. Here are
// default configuration values which will be merged with values read from
// JSON configuration file (if configuration file is found).
var Conf = Config{
	Cmd: Command{
		Viewer: "less", Editor: "vi", Diff: "diff"},
	Log:                 Log{MaxEntrySize: logging.MaxEntrySize},
	Sync:                Sync{Seconds: 4},
	DataPowerAppliances: make(map[string]DataPowerAppliance)}

// k is Confident library configuration instance.
var k *confident.Confident

// initConfiguration reads dpcmder configuration and defines DataPower appliance
// to use.
func initConfiguration() {
	k = confident.New()
	k.WithConfiguration(&Conf)
	k.Name = "config"
	k.Type = "json"
	k.Path = configDirPath()
	k.Path = configDirPathEnsureExists()
	k.Permission = os.FileMode(0644)
	logging.LogDebugf("config/initConfiguration() - Conf before read: %#v", Conf)
	k.Read()
	logging.LogDebugf("config/initConfiguration() - Conf after read: %#v", Conf)
	if *dpRestURL != "" || *dpSomaURL != "" {
		if *dpConfigName != "" {
			validateDpConfigName()
			Conf.DataPowerAppliances[*dpConfigName] = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
			CurrentApplianceName = *dpConfigName
		} else {
			Conf.DataPowerAppliances[PreviousApplianceName] = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
			CurrentApplianceName = PreviousApplianceName
		}
		k.Persist()
		logging.LogDebugf("config/initConfiguration() - Conf after persist: %#v", Conf)
	}
	CurrentAppliance = DataPowerAppliance{Domain: *dpDomain, Proxy: *proxy, RestUrl: *dpRestURL, SomaUrl: *dpSomaURL, Username: *dpUsername, Password: *dpPassword}
}

// parseProgramArgs parses program arguments and fill config package variables with flag values.
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
	helpUsage = flag.Bool("h", false, "Show dpcmder usage with examples")
	helpFull = flag.Bool("help", false, "Show dpcmder in-program help on console")
	version = flag.Bool("v", false, "Show dpcmder version")

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
	initConfiguration()
	validatePassword()
}

// validateProgramArgs validate parsed program arguments and/or shows usage
// message in case some mandatory arguments are missing.
func validateProgramArgs() {
	if *version {
		showVersion()
	}

	if *helpUsage {
		usage(0)
	}

	if *helpFull {
		showHelp(0)
	}

	if *LocalFolderPath == "" ||
		(*dpUsername != "" && *dpRestURL == "" && *dpSomaURL == "") {
		usage(1)
	}

}

// validateDpConfigName validate DataPower appliance configuration name passed
// as command line param to avoid overwritting of existing configuration.
func validateDpConfigName() {
	if _, ok := Conf.DataPowerAppliances[*dpConfigName]; ok {
		fmt.Printf("DataPower appliance configuration with name '%s' already exists.\n\n", *dpConfigName)
		usage(2)
	}
}

// validatePassword validates password argument and asks for password input
// and/or shows usage message in case it is missing - need to call it after
// initial command line paramter reading to avoid saving password entered during
// dpcmder start to config file.
func validatePassword() {
	if *dpUsername != "" {
		if *dpPassword == "" {
			fmt.Println("DataPower password: ")
			// Silent. For printing *'s use crypto terminal.GetPasswdMasked()
			pass, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				usage(1)
			} else {
				password := string(pass)
				setDpPasswordPlain(password)
				DpTransientPasswordMap[CurrentApplianceName] = password
			}
		}

		if *dpPassword == "" {
			fmt.Println("Password can't be empty!")
			fmt.Println()
			usage(1)
		}
	}
}

// configDirPath returns dpcmder configuration directory path.
func configDirPath() string {
	usr, err := user.Current()
	if err != nil {
		logging.LogFatal("config/configDirPath() - Can't find current user: ", err)
	}

	// println("usr.HomeDir: ", usr.HomeDir)
	configDirPath := paths.GetFilePath(usr.HomeDir, configDirName)

	return configDirPath
}

// configDirPathEnsureExists returns dpcmder configuration directory path and
// in case it doesn't exist creates directory.
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

// setDpPasswordPlain sets config dpPassword encoded password field from
// plaintext password.
func setDpPasswordPlain(password string) {
	b32password := base32.StdEncoding.EncodeToString([]byte(password))
	dpPassword = &b32password
}

// CreateDpApplianceConfig creates empty DataPower appliance JSON configuration as byte array.
func (c *Config) CreateDpApplianceConfig() ([]byte, error) {
	dpAppliance := DataPowerAppliance{}
	json, err := json.MarshalIndent(dpAppliance, "", "  ")
	if err != nil {
		logging.LogDebugf(
			"config/CreateDpApplianceConfig() - Can't marshal empty DataPower appliance configuration (%v).",
			dpAppliance)
		return nil, err
	}
	return json, nil
}

// GetDpApplianceConfig fetches DataPower appliance JSON configuration as byte array.
func (c *Config) GetDpApplianceConfig(name string) ([]byte, error) {
	dpAppliance := c.DataPowerAppliances[name]
	json, err := json.MarshalIndent(dpAppliance, "", "  ")
	if err != nil {
		logging.LogDebugf("config/GetDpApplianceConfig('%s') - Can't marshal DataPower appliance configuration.", name)
		return nil, err
	}
	return json, nil
}

// SetDpApplianceConfig sets DataPower appliance JSON configuration as byte array.
func (c *Config) SetDpApplianceConfig(name string, contents []byte) error {
	dpAppliance := c.DataPowerAppliances[name]
	err := json.Unmarshal(contents, &dpAppliance)
	if err != nil {
		logging.LogDebugf("config/SetDpApplianceConfig('%s', ...) - Can't unmarshal DataPower appliance configuration,", name)
		return err
	}
	c.DataPowerAppliances[name] = dpAppliance
	k.Persist()
	return nil
}

// DeleteDpApplianceConfig deletes DataPower appliance JSON configuration.
func (c *Config) DeleteDpApplianceConfig(name string) {
	delete(c.DataPowerAppliances, name)
	k.Persist()
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
	fmt.Println("helpUsage: ", *helpUsage)
	fmt.Println("helpFull: ", *helpFull)
	fmt.Println("version: ", *version)
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
	logging.LogDebug("helpUsage: ", *helpUsage)
	logging.LogDebug("helpFull: ", *helpFull)
	logging.LogDebug("version: ", *version)
}

// usage prints usage help information with examples to console.
func usage(exitStatus int) {
	fmt.Println("Usage:")
	fmt.Printf(" %s [-l LOCAL_FOLDER_PATH] [-r DATA_POWER_REST_URL | -s DATA_POWER_SOMA_AMP_URL] [-u USERNAME] [-p PASSWORD] [-d DP_DOMAIN] [-x PROXY_SERVER] [-c DP_CONFIG_NAME] [-debug] [-h] [-help]\n", os.Args[0])
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
	fmt.Println(" -trace - turns on creation of dpcmder.log file with trace log messages")
	fmt.Println(" -h - shows this (usage) help")
	fmt.Println(" -help - shows dpcmder full help on console")
	fmt.Println(" -v - shows dpcmder version")
	fmt.Println("")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Printf(" %s\n", os.Args[0])
	fmt.Println("   - runs dpcmder with available DataPower connection configurations shown")
	fmt.Printf(" %s -help\n", os.Args[0])
	fmt.Println("   - shows full dpcmder help")
	fmt.Printf(" %s -r https://localhost:5554 -u admin\n", os.Args[0])
	fmt.Println("   - connect to DataPower using REST managment interface and ask for password")
	fmt.Printf(" %s -s https://localhost:5550 -u admin -p admin -d default -debug\n", os.Args[0])
	fmt.Println("   - connect to DataPower using SOMA managment interface and write debug messages to ./dpcmder.log file")
	fmt.Printf(" %s -s https://localhost:5550 -u admin -p admin -c LocalDp\n", os.Args[0])
	fmt.Println("   - connect to DataPower using SOMA managment interface and save configuration parameters as LocalDp")

	os.Exit(exitStatus)
}

func showVersion() {
	fmt.Printf("dpcmder version %s %s %s\n", help.Version, help.Platform, help.BuildTime)

	os.Exit(0)
}

// help prints in-program help information to console.
func showHelp(exitStatus int) {
	fmt.Print(help.Help)

	os.Exit(exitStatus)
}
