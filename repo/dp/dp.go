// Package dp implements access to DataPower appliances.
package dp

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/clbanning/mxj"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"github.com/savaki/jq"
)

// dpApplicance extends with additional name field. This struct contains all
// DataPower configuration info from config module + additional info required
// to communicate with appliance.
type dpApplicance struct {
	name string
	config.DataPowerAppliance
}

// dpRepo contains basic DataPower repo information and implements Repo interface.
type dpRepo struct {
	name               string
	dpFilestoreXmls    map[string]string
	invalidateCache    bool
	dataPowerAppliance dpApplicance
	DpViewMode         model.DpViewMode
	req                requester
}

// Repo is instance or DataPower repo/Repo interface implementation used for all
// operations on DataPower except syncing local filesystem to DataPower.
var Repo = dpRepo{name: "DataPower", dpFilestoreXmls: make(map[string]string),
	DpViewMode: model.DpFilestoreMode, req: netRequester{}}

// SyncRepo is instance or DataPower repo/Repo interface implementation used for
// syncing local directory to DataPower directory.
var SyncRepo = dpRepo{name: "SyncDataPower", dpFilestoreXmls: make(map[string]string),
	DpViewMode: model.DpFilestoreMode, req: netRequester{}}

// dpDomainInfo contains domain name and basic state
type dpDomainInfo struct {
	name       string
	saveNeeded bool
	down       bool
}

// Constants from xml-mgmt.xsd (dmConfigState type), only used ones.
const (
	objectStatusSaved    = "saved"
	objectStatusExternal = "external"
)

func (r *dpRepo) String() string {
	return r.name
}

func (r *dpRepo) GetInitialItem() (model.Item, error) {
	logging.LogDebugf("repo/dp/GetInitialItem(), dataPowerAppliance: %#v", r.dataPowerAppliance)
	var initialConfig model.ItemConfig
	initialViewName := "List appliance configurations"
	var initialConfigType model.ItemType
	initialConfigTop := model.ItemConfig{Type: model.ItemNone}
	if r.dataPowerAppliance.RestUrl != "" || r.dataPowerAppliance.SomaUrl != "" || r.dataPowerAppliance.Username != "" {
		if r.dataPowerAppliance.Domain != "" {
			initialViewName = r.dataPowerAppliance.Domain
			initialConfigType = model.ItemDpDomain
		} else {
			initialViewName = r.dataPowerAppliance.name
			initialConfigType = model.ItemDpConfiguration
		}
		initialConfig = model.ItemConfig{
			Type:        initialConfigType,
			Name:        initialViewName,
			DpAppliance: r.dataPowerAppliance.name,
			DpDomain:    r.dataPowerAppliance.Domain,
			Parent:      &initialConfigTop}
	} else {
		initialConfig = initialConfigTop
	}
	logging.LogDebugf("repo/dp/GetInitialItem() initialConfig: %#v", initialConfig)
	initialItem := model.Item{Name: initialViewName, Config: &initialConfig}

	return initialItem, nil
}

func (r *dpRepo) GetTitle(itemToShow *model.ItemConfig) string {
	logging.LogDebugf("repo/dp/GetTitle(%v)", itemToShow)
	dpConfigName := itemToShow.DpAppliance
	dpDomain := itemToShow.DpDomain
	currPath := itemToShow.Path

	var url string
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		url = r.dataPowerAppliance.RestUrl
	case config.DpInterfaceSoma:
		url = r.dataPowerAppliance.SomaUrl
	default:
		logging.LogDebug("repo/dp/GetTitle(), using neither REST neither SOMA.")
	}

	return fmt.Sprintf("%s @ %s - %s (%s) %s",
		r.dataPowerAppliance.Username, url, dpConfigName, dpDomain, currPath)
}
func getDpAppliance(itemToShow *model.ItemConfig) dpApplicance {
	switch itemToShow.Type {
	case model.ItemNone:
		return dpApplicance{}
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore,
		model.ItemDpObjectClassList, model.ItemDpObjectClass,
		model.ItemDpStatusClassList, model.ItemDpStatusClass,
		model.ItemDirectory:
		dataPowerAppliance := config.Conf.DataPowerAppliances[itemToShow.DpAppliance]
		if dataPowerAppliance.Password == "" {
			dataPowerAppliance.SetDpPlaintextPassword(config.DpTransientPasswordMap[itemToShow.DpAppliance])
		}
		return dpApplicance{name: itemToShow.DpAppliance,
			DataPowerAppliance: dataPowerAppliance}
	default:
		logging.LogDebugf("repo/dp/getDpAppliance(%v) - unknown item type: %v.",
			itemToShow, itemToShow.Type)
		return dpApplicance{}
	}
}
func (r *dpRepo) GetList(itemToShow *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/GetList(%v), r.DpViewMode: %s", itemToShow, r.DpViewMode)

	switch r.DpViewMode {
	case model.DpObjectMode:
		if itemToShow.DpAppliance == "" {
			logging.LogDebugf("repo/dp/GetList(%v) - can't find DpAppliance.", itemToShow)
			return nil, errs.Errorf("Internal error showing object config mode - missing dp appliance.")
		}

		if itemToShow.DpDomain == "" {
			logging.LogDebugf("repo/dp/GetList(%v) - can't find DpDomain.", itemToShow)
			return nil, errs.Errorf("Internal error showing object config mode - missing domain.")
		}

		switch itemToShow.Type {
		case model.ItemDpObjectClassList:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listObjectClasses(itemToShow)
		case model.ItemDpObjectClass:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listObjects(itemToShow)
		default:
			logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item for DpViewMode: %s.",
				itemToShow, r.DpViewMode)
			r.DpViewMode = model.DpFilestoreMode
			return nil, errs.Errorf("Internal error showing object config mode - wrong view type.")
		}
	case model.DpStatusMode:
		if itemToShow.DpAppliance == "" {
			logging.LogDebugf("repo/dp/GetList(%v) - can't find DpAppliance.", itemToShow)
			return nil, errs.Errorf("Internal error showing status config mode - missing dp appliance.")
		}

		if itemToShow.DpDomain == "" {
			logging.LogDebugf("repo/dp/GetList(%v) - can't find DpDomain.", itemToShow)
			return nil, errs.Errorf("Internal error showing status config mode - missing domain.")
		}

		switch itemToShow.Type {
		case model.ItemDpStatusClassList:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listStatusClasses(itemToShow)
		case model.ItemDpStatusClass:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listStatuses(itemToShow)
		default:
			wrongView := r.DpViewMode
			logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item for DpViewMode: %s.",
				itemToShow, wrongView)
			r.DpViewMode = model.DpFilestoreMode
			return nil, errs.Errorf(
				"Internal error showing status config mode - wrong view type: %s.",
				wrongView)
		}

	case model.DpFilestoreMode:
		switch itemToShow.Type {
		case model.ItemNone:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return listAppliances()
		case model.ItemDpConfiguration:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			if itemToShow.DpDomain != "" {
				return r.listFilestores(itemToShow)
			}
			return r.listDomains(itemToShow)
		case model.ItemDpDomain:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listFilestores(itemToShow)
		case model.ItemDpFilestore:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listDpDir(itemToShow)
		case model.ItemDirectory:
			r.dataPowerAppliance = getDpAppliance(itemToShow)
			return r.listDpDir(itemToShow)
		default:
			logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item for DpViewMode: %s.",
				itemToShow, r.DpViewMode)
			return nil, errs.Errorf("Internal error showing filestore mode - wrong view type.")
		}
	default:
		return nil, errs.Errorf("Unknown DataPower view mode.")
	}
}

func (r *dpRepo) InvalidateCache() {
	logging.LogDebugf("repo/dp/InvalidateCache()")
	if r.dataPowerAppliance.SomaUrl != "" {
		r.invalidateCache = true
	}
}

func (r *dpRepo) GetFile(currentView *model.ItemConfig, fileName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetFile(%v, '%s')", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetDpPath(parentPath, fileName)
	r.dataPowerAppliance = getDpAppliance(currentView)

	return r.GetFileByPath(currentView.DpDomain, filePath)
}

// GetFileByPath fetches file from DataPower by it's domain and path.
func (r *dpRepo) GetFileByPath(dpDomain, filePath string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetFile('%s', '%s')", dpDomain, filePath)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		restPath := makeRestPath(dpDomain, filePath)

		fileB64, _, err := r.restGetForOneResult(restPath, "/file")
		if err != nil {
			return nil, err
		}

		resultBytes, err := base64.StdEncoding.DecodeString(fileB64)
		if err != nil {
			logging.LogDebug("repo/dp/GetFile() - Error decoding base64 file.", err)
			return nil, err
		}

		return resultBytes, nil
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:get-file name="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, dpDomain, filePath)
		somaResponse, err := r.soma(somaRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("repo/dp/GetFile() - Error parsing response SOAP.", err)
			return nil, err
		}
		fileNode := xmlquery.FindOne(doc, "//*[local-name()='file']")

		if fileNode == nil {
			errMsg := fmt.Sprintf("Can't find file '%s' from SOMA response.", filePath)
			logging.LogDebug(errMsg)
			return nil, errs.Error(errMsg)
		}
		fileB64 := fileNode.InnerText()
		resultBytes, err := base64.StdEncoding.DecodeString(fileB64)
		if err != nil {
			logging.LogDebug("repo/dp/GetFile() - Error decoding base64 file.", err)
			return nil, err
		}

		return resultBytes, nil
	default:
		logging.LogDebug("repo/dp/GetFile(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

func (r *dpRepo) UpdateFile(currentView *model.ItemConfig, fileName string, newFileContent []byte) (bool, error) {
	logging.LogDebugf("repo/dp/UpdateFile(%s, '%s', ...)\n", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetDpPath(parentPath, fileName)
	r.dataPowerAppliance = getDpAppliance(currentView)
	return r.UpdateFileByPath(currentView.DpDomain, filePath, newFileContent)
}
func (r *dpRepo) UpdateFileByPath(dpDomain, filePath string, newFileContent []byte) (bool, error) {
	logging.LogDebugf("repo/dp/UpdateFileByPath('%s', '%s', ...)", dpDomain, filePath)
	fileType, err := r.GetFileTypeByPath(dpDomain, filePath, ".")
	logging.LogDebugf("repo/dp/UpdateFileByPath() fileType: %s", fileType)
	if err != nil {
		return false, err
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		updateFilePath := ""
		restMethod := ""
		switch fileType {
		case model.ItemNone:
			updateFilePath = paths.GetDpPath(filePath, "..")
			restMethod = "POST"
		case model.ItemFile:
			updateFilePath = filePath
			restMethod = "PUT"
		case model.ItemDirectory:
			errMsg := fmt.Sprintf("Can't upload file '%s', directory with same name exists.", filePath)
			logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		default:
			errMsg := fmt.Sprintf("Can't upload file '%s', type '%s' with same name exists.", filePath, fileType)
			logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		}

		_, fileName := splitOnLast(filePath, "/")
		requestBody := "{\"file\":{\"name\":\"" + fileName + "\",\"content\":\"" + base64.StdEncoding.EncodeToString(newFileContent) + "\"}}"

		restPath := makeRestPath(dpDomain, updateFilePath)
		jsonString, err := r.rest(restPath, restMethod, requestBody)
		if err != nil {
			return false, err
		}

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogDebug("repo/dp/UpdateFileByPath() - Error parsing response JSON.", err)
			return false, err
		}

		jsonError := jsonquery.Find(doc, "/error")
		if len(jsonError) != 0 {
			errMsg := fmt.Sprintf("Uploading file '%s', returned '%s'.", filePath, jsonString)
			logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
			return false, err
		}

		return true, nil
	case config.DpInterfaceSoma:
		switch fileType {
		case model.ItemNone, model.ItemFile:
			somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:set-file name="%s">%s</man:set-file>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, dpDomain, filePath, base64.StdEncoding.EncodeToString(newFileContent))
			somaResponse, err := r.soma(somaRequest)
			if err != nil {
				return false, err
			}
			doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
			if err != nil {
				logging.LogDebug("repo/dp/UpdateFileByPath() - Error parsing response SOAP.", err)
				return false, err
			}
			parentPath := paths.GetDpPath(filePath, "..")
			err = r.refreshSomaFilesByPath(dpDomain, parentPath)
			if err != nil {
				logging.LogDebugf("repo/dp/UpdateFileByPath() - Error refresing soma files by path '%s': err: %v", parentPath, err)
				return false, err
			}
			resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
			if resultNode == nil {
				errMsg := fmt.Sprintf("Error refresing soma files by path '%s': err: %v", parentPath, err)
				logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
				return false, errs.Error(errMsg)
			}
			resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
			if resultText != "OK" {
				errMsg := fmt.Sprintf("Unexpected result of refresh soma files by path '%s': result: '%s'", parentPath, resultText)
				logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
				return false, errs.Error(errMsg)
			}
			return true, nil
		case model.ItemDirectory:
			errMsg := fmt.Sprintf("Can't upload file '%s', directory with same name exists.", filePath)
			logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		default:
			errMsg := fmt.Sprintf("Can't upload file '%s', type '%s' with same name exists.", filePath, fileType)
			logging.LogDebugf("repo/dp/UpdateFileByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		}
	default:
		logging.LogDebug("repo/dp/UpdateFileByPath(), using neither REST neither SOMA.")
		return false, errs.Error("DataPower management interface not set.")
	}
}

func (r *dpRepo) GetFileType(viewConfig *model.ItemConfig, parentPath, fileName string) (model.ItemType, error) {
	logging.LogDebug(fmt.Sprintf("repo/dp/getFileType(%v, '%s', '%s')\n", viewConfig, parentPath, fileName))
	dpDomain := viewConfig.DpDomain
	r.dataPowerAppliance = getDpAppliance(viewConfig)

	return r.GetFileTypeByPath(dpDomain, parentPath, fileName)
}

func (r *dpRepo) GetFileTypeByPath(dpDomain, parentPath, fileName string) (model.ItemType, error) {
	logging.LogDebug(fmt.Sprintf("repo/dp/GetFileTypeByPath('%s', '%s', '%s')\n", dpDomain, parentPath, fileName))
	filePath := paths.GetDpPath(parentPath, fileName)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		restPath := makeRestPath(dpDomain, filePath)
		jsonString, err := r.restGet(restPath)
		if err != nil {
			unexErr, ok := err.(errs.UnexpectedHTTPResponse)
			if ok && unexErr.StatusCode == 404 {
				return model.ItemNone, nil
			}
			return model.ItemNone, err
		}

		if jsonString == "" {
			return model.ItemNone, nil
		}

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogDebug("Error parsing response JSON.", err)
			return model.ItemNone, err
		}

		filestore := jsonquery.Find(doc, "/filestore")
		locationName := jsonquery.Find(doc, "/filestore/location/name")
		file := jsonquery.Find(doc, "/file")
		switch {
		case len(locationName) == 1 && strings.HasSuffix(locationName[0].InnerText(), ":"):
			return model.ItemDpFilestore, nil
		case len(filestore) == 1:
			return model.ItemDirectory, nil
		case len(file) == 1:
			return model.ItemFile, nil
		case len(file) == 0:
			return model.ItemNone, nil
		}

		errMsg := fmt.Sprintf("Wrong JSON response: '%s'", jsonString)
		logging.LogDebugf("repo/dp/GetFileTypeByPath() - %s", errMsg)
		return model.ItemNone, errs.Error(errMsg)
	case config.DpInterfaceSoma:
		switch {
		case parentPath != "":
			dpFilestoreLocation, _ := splitOnFirst(parentPath, "/")
			if !strings.HasSuffix(dpFilestoreLocation, ":") {
				dpFilestoreLocation = dpFilestoreLocation + ":"
			}
			dpFilestoreIsRoot := !strings.Contains(parentPath, "/")
			var dpDirNodes []*xmlquery.Node
			var dpFileNodes []*xmlquery.Node
			if dpFilestoreIsRoot {
				doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXmls[dpFilestoreLocation]))
				if err != nil {
					logging.LogDebug("Error parsing response SOAP.", err)
					return model.ItemNone, err
				}
				dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory[@name='"+filePath+"']")
				dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file[@name='"+fileName+"']")
			} else {
				doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXmls[dpFilestoreLocation]))
				if err != nil {
					logging.LogDebug("Error parsing response SOAP.", err)
					return model.ItemNone, err
				}
				dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+filePath+"']")
				dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+parentPath+"']/file[@name='"+fileName+"']")
			}

			switch {
			case len(dpDirNodes) == 1:
				return model.ItemDirectory, nil
			case len(dpFileNodes) == 1:
				return model.ItemFile, nil
			case len(dpFileNodes) == 0:
				return model.ItemNone, nil
			default:
				errMsg := fmt.Sprintf("Wrong SOAP response: '%s'", r.dpFilestoreXmls[dpFilestoreLocation])
				logging.LogDebugf("repo/dp/GetFileTypeByPath() - %s", errMsg)
				return model.ItemNone, errs.Error(errMsg)
			}

		case dpDomain != "":
			return model.ItemDpFilestore, nil
		default:
			return model.ItemDpDomain, nil
		}
	default:
		logging.LogDebug("repo/dp/GetFileTypeByPath(), using neither REST neither SOMA.")
		return model.ItemNone, errs.Error("DataPower management interface not set.")
	}
}

func (r *dpRepo) GetFilePath(parentPath, fileName string) string {
	logging.LogDebugf("repo/dp/GetFilePath('%s', '%s')", parentPath, fileName)
	return paths.GetDpPath(parentPath, fileName)
}

func (r *dpRepo) CreateDir(viewConfig *model.ItemConfig, parentPath, dirName string) (bool, error) {
	logging.LogDebugf("repo/dp/CreateDir(%v, '%s', '%s')", viewConfig, parentPath, dirName)
	return r.CreateDirByPath(viewConfig.DpDomain, parentPath, dirName)
}
func (r *dpRepo) CreateDirByPath(dpDomain, parentPath, dirName string) (bool, error) {
	logging.LogDebugf("repo/dp/CreateDirByPath('%s', '%s', '%s')", dpDomain, parentPath, dirName)
	fileType, err := r.GetFileTypeByPath(dpDomain, parentPath, dirName)
	if err != nil {
		return false, err
	}

	switch fileType {
	case model.ItemNone:
		switch r.dataPowerAppliance.DpManagmentInterface() {
		case config.DpInterfaceRest:
			requestBody := "{\"directory\":{\"name\":\"" + dirName + "\"}}"
			restPath := makeRestPath(dpDomain, parentPath)
			jsonString, err := r.rest(restPath, "POST", requestBody)
			if err != nil {
				return false, err
			}
			// println("jsonString: " + jsonString)

			doc, err := jsonquery.Parse(strings.NewReader(jsonString))
			if err != nil {
				logging.LogDebug("Error parsing response JSON.", err)
				return false, err
			}

			error := jsonquery.Find(doc, "/error")
			if len(error) == 0 {
				return true, nil
			}
			errMsg := fmt.Sprintf("Can't create dir '%s' at '%s', json returned : '%s'.", dirName, parentPath, jsonString)
			logging.LogDebugf("repo/dp/CreateDirByPath() - %v", errMsg)
			return false, errs.Error(errMsg)
		case config.DpInterfaceSoma:
			dirPath := r.GetFilePath(parentPath, dirName)
			somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:do-action><CreateDir><Dir>%s</Dir></CreateDir></man:do-action>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, dpDomain, dirPath)
			somaResponse, err := r.soma(somaRequest)
			if err != nil {
				return false, err
			}
			doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
			if err != nil {
				logging.LogDebug("Error parsing response SOAP.", err)
				return false, err
			}
			r.refreshSomaFilesByPath(dpDomain, dirPath)
			resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
			if resultNode != nil {
				resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
				if resultText == "OK" {
					return true, nil
				}
			}
			errMsg := fmt.Sprintf("Error creating '%s' dir in path '%s'", dirName, parentPath)
			logging.LogDebugf("repo/dp/CreateDirByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		default:
			logging.LogDebug("repo/dp/CreateDirByPath(), using neither REST neither SOMA.")
			return false, errs.Error("DataPower management interface not set.")
		}
	case model.ItemDirectory:
		return true, nil
	default:
		errMsg := fmt.Sprintf("Can't create dir '%s' at '%s' (%v) with same name exists.", dirName, parentPath, fileType)
		logging.LogDebugf("repo/dp/CreateDirByPath() - %s", errMsg)
		return false, nil
	}
}

func (r *dpRepo) Delete(currentView *model.ItemConfig, itemType model.ItemType, parentPath, fileName string) (bool, error) {
	logging.LogDebugf("repo/dp/Delete(%v, '%s', '%s' (%s))", currentView, parentPath, fileName, itemType)

	switch itemType {
	case model.ItemDpConfiguration:
		// deleting DataPower configuration
		config.Conf.DeleteDpApplianceConfig(fileName)
		return true, nil
	case model.ItemDirectory, model.ItemFile:
		filePath := r.GetFilePath(parentPath, fileName)

		switch r.dataPowerAppliance.DpManagmentInterface() {
		case config.DpInterfaceRest:
			restPath := makeRestPath(currentView.DpDomain, filePath)
			jsonString, err := r.rest(restPath, "DELETE", "")
			if err != nil {
				return false, err
			}
			// println("jsonString: " + jsonString)

			doc, err := jsonquery.Parse(strings.NewReader(jsonString))
			if err != nil {
				logging.LogDebug("Error parsing response JSON.", err)
				return false, err
			}

			error := jsonquery.Find(doc, "/error")
			if len(error) == 0 {
				return true, nil
			}
		case config.DpInterfaceSoma:
			fileType, err := r.GetFileType(currentView, parentPath, fileName)
			if err != nil {
				return false, err
			}
			var somaRequest string
			switch fileType {
			case model.ItemDirectory:
				somaRequest = fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:do-action><RemoveDir><Dir>%s</Dir></RemoveDir></man:do-action>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, currentView.DpDomain, filePath)
			case model.ItemFile:
				somaRequest = fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:do-action><DeleteFile><File>%s</File></DeleteFile></man:do-action>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, currentView.DpDomain, filePath)
			}
			somaResponse, err := r.soma(somaRequest)
			if err != nil {
				return false, err
			}
			doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
			if err != nil {
				logging.LogDebug("Error parsing response SOAP.", err)
				return false, err
			}
			r.refreshSomaFiles(currentView)
			resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
			if resultNode != nil {
				resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
				if resultText == "OK" {
					return true, nil
				}
			}
		default:
			logging.LogDebug("repo/dp/Delete(), using neither REST neither SOMA.")
			return false, errs.Error("DataPower management interface not set.")
		}
	case model.ItemDpObject:
		switch r.dataPowerAppliance.DpManagmentInterface() {
		case config.DpInterfaceRest:
			restPath := fmt.Sprintf("/mgmt/config/%s/%s/%s", currentView.DpDomain, parentPath, fileName)
			logging.LogDebugf("repo/dp/Delete(), restPath: '%s'", restPath)
			jsonString, err := r.rest(restPath, "DELETE", "")
			if err != nil {
				return false, err
			}
			logging.LogDebugf("jsonString: '%s'", jsonString)
			resultMsg, err := parseJSONFindOne(jsonString, fmt.Sprintf("/%s", fileName))
			if err != nil {
				return false, err
			}
			if resultMsg == "Configuration was deleted." {
				return true, nil
			}
		case config.DpInterfaceSoma:
			somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:del-config><%s name="%s"/></man:del-config>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`,
				currentView.DpDomain, parentPath, fileName)
			somaResponse, err := r.soma(somaRequest)
			if err != nil {
				return false, err
			}
			result, err := parseSOMAFindOne(somaResponse, "//*[local-name()='response']/*[local-name()='result']")
			if err != nil {
				logging.LogDebug("Error parsing response SOAP.", err)
				return false, err
			}
			if result == "OK" {
				return true, nil
			}
			return false, errs.Error(result)
		default:
			logging.LogDebug("repo/dp/Delete(), using neither REST neither SOMA.")
			return false, errs.Error("DataPower management interface not set.")
		}
	default:
		logging.LogDebugf("repo/dp/Delete(), don't know how to delete item type %s.", itemType)
		return false, errs.Errorf("Don't know how to delete item type %s.", itemType.UserFriendlyString())
	}
	return false, errs.Errorf("Can't delete '%s' (%s) at path '%s'.", fileName, itemType.UserFriendlyString(), parentPath)
}

func (r *dpRepo) GetViewConfigByPath(currentView *model.ItemConfig, dirPath string) (*model.ItemConfig, error) {
	logging.LogDebugf("repo/dp/GetViewConfigByPath('%s')", dirPath)
	if currentView.DpDomain == "" {
		return nil, errs.Errorf("Can't get view for path '%s' if DataPower domain is not selected.", dirPath)
	}

	dpView := currentView
	for dpView.Type != model.ItemDpDomain {
		dpView = dpView.Parent
	}

	resultView := dpView
	parentView := dpView
	dirPath = strings.TrimRight(dirPath, "/")
	dirPathElements := paths.SplitDpPath(dirPath)
	for idx, dirFsName := range dirPathElements {
		itemType := model.ItemDirectory
		dpFilestore := parentView.DpFilestore
		if idx == 0 {
			itemType = model.ItemDpFilestore
			dpFilestore = dirFsName
			parentView = dpView
		}
		resultView = &model.ItemConfig{Type: itemType,
			Name:        dirFsName,
			Path:        paths.GetDpPath(parentView.Path, dirFsName),
			DpAppliance: dpView.DpAppliance,
			DpDomain:    dpView.DpDomain,
			DpFilestore: dpFilestore,
			Parent:      parentView}
		parentView = resultView
	}

	return resultView, nil
}

// ExportAppliance creates export of whole DataPower appliance and returns
// base64 encoded exported zip file.
func (r *dpRepo) ExportAppliance(applianceConfigName, exportFileName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/ExportAppliance('%s', '%s')", applianceConfigName, exportFileName)

	// 0. Prepare DataPower connection configuration.
	oldDataPowerAppliance := r.dataPowerAppliance
	// r.dataPowerAppliance = config.Conf.DataPowerAppliances[applianceConfigName]
	r.dataPowerAppliance = dpApplicance{name: applianceConfigName,
		DataPowerAppliance: config.Conf.DataPowerAppliances[applianceConfigName]}
	clearCurrentConfig := func() {
		r.dataPowerAppliance = oldDataPowerAppliance
	}
	defer clearCurrentConfig()
	if r.dataPowerAppliance.Password == "" {
		r.dataPowerAppliance.SetDpPlaintextPassword(config.DpTransientPasswordMap[applianceConfigName])
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// Don't know how to export multiple domains using REST.
		return nil,
			errs.Errorf("DataPower management interface %s not supported for appliance export.",
				r.dataPowerAppliance.DpManagmentInterface())
	case config.DpInterfaceSoma:
		// 1. Fetch export (backup) of all domains
		//    Backup contains all domains export zip + export info and dp-aux files
		domains, err := r.fetchDpDomains()
		if err != nil {
			return nil, err
		}
		logging.LogDebugf("repo/dp/ExportAppliance(), domainNames: %v", domains)

		backupRequestSomaDomains := ""
		for _, domain := range domains {
			backupRequestSomaDomains = backupRequestSomaDomains + fmt.Sprintf(`<man:domain name="%s"/>`, domain.name)
		}
		backupRequestSoma := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request>
			<man:do-backup format="ZIP" persisted="false">
				<man:user-comment>Created by dpcmder - %s</man:user-comment>
%s
			</man:do-backup>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, exportFileName, backupRequestSomaDomains)
		backupResponseSoma, err := r.soma(backupRequestSoma)
		if err != nil {
			return nil, err
		}
		backupFileB64, err := parseSOMAFindOne(backupResponseSoma, "//*[local-name()='file']")
		if err != nil {
			return nil, err
		}

		backupBytes, err := base64.StdEncoding.DecodeString(backupFileB64)
		if err != nil {
			logging.LogDebug("repo/dp/ExportDomain() - Error decoding base64 file.", err)
			return nil, err
		}

		return backupBytes, nil
	default:
		return nil, errs.Errorf("DataPower management interface %s not supported.", r.dataPowerAppliance.DpManagmentInterface())
	}
}

// ExportDomain creates export of given domain and returns base64 encoded
// exported zip file.
func (r *dpRepo) ExportDomain(domainName, exportFileName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/ExportDomain('%s', '%s')", domainName, exportFileName)
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// 1. Start export (send export request)
		exportRequestJSON := fmt.Sprintf(`{"Export":
		  {
		    "Format":"ZIP",
		    "UserComment":"Created by dpcmder - %s.",
		    "AllFiles":"on",
		    "Persisted":"off",
		    "IncludeInternalFiles":"off"
		  }
		}`, exportFileName)
		locationURL, _, err := r.restPostForResult(
			"/mgmt/actionqueue/"+domainName,
			exportRequestJSON,
			"/Export/status",
			"Action request accepted.",
			"/_links/location/href")
		if err != nil {
			return nil, err
		}

		timeStart := time.Now()
		for {
			// 2. Check for current status of export request
			status, exportResponseJSON, err := r.restGetForOneResult(locationURL, "/status")
			logging.LogDebugf("repo/dp/ExportDomain() status: '%s'", status)
			if err != nil {
				return nil, err
			}

			switch status {
			case "started":
				if time.Since(timeStart) > 120*time.Second {
					logging.LogDebugf("repo/dp/ExportDomain() waiting for export since %v, giving up.\n last exportResponseJSON: '%s'", timeStart, exportResponseJSON)
					return nil, errs.Errorf("Export didn't finish since %v, giving up.", timeStart)
				}
				time.Sleep(1 * time.Second)
			case "completed":
				// 3. When export is completed get base64 result file from it
				logging.LogDebugf("repo/dp/ExportDomain() export fetched after %d.", time.Since(timeStart))
				fileB64, err := parseJSONFindOne(exportResponseJSON, "/result/file")
				if err != nil {
					return nil, err
				}
				fileBytes, err := base64.StdEncoding.DecodeString(fileB64)
				return fileBytes, err
			default:
				return nil, errs.Errorf("Unexpected response from server ('%s').", status)
			}
		}
	case config.DpInterfaceSoma:
		// 1. Fetch export (backup) of domain
		//    Backup contains domain export zip + export info and dp-aux files
		backupRequestSoma := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request>
			<man:do-backup format="ZIP" persisted="false">
				<man:user-comment>Created by dpcmder - %s</man:user-comment>
				<man:domain name="%s"/>
			</man:do-backup>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, exportFileName, domainName)
		backupResponseSoma, err := r.soma(backupRequestSoma)
		if err != nil {
			return nil, err
		}
		backupFileB64, err := parseSOMAFindOne(backupResponseSoma, "//*[local-name()='file']")
		if err != nil {
			return nil, err
		}

		backupBytes, err := base64.StdEncoding.DecodeString(backupFileB64)
		if err != nil {
			logging.LogDebug("repo/dp/ExportDomain() - Error decoding base64 file.", err)
			return nil, err
		}

		// 2. Extract just the received domain backup archive
		backupBytesReader := bytes.NewReader(backupBytes)
		backupZipReader, err := zip.NewReader(backupBytesReader, int64(len(backupBytes)))
		if err != nil {
			logging.LogDebug("repo/dp/ExportDomain() - Error unzipping backup archive.", err)
			return nil, err
		}
		for idx, file := range backupZipReader.File {
			logging.LogDebugf("repo/dp/ExportDomain() - file[%d] : '%s'.", idx, file.Name)
			if file.Name == domainName+".zip" {
				domainReader, err := file.Open()
				if err != nil {
					logging.LogDebug("repo/dp/ExportDomain() - Error opening domain from backup archive for reading.", err)
					return nil, err
				}
				defer domainReader.Close()

				domainBackupBytes := make([]byte, file.UncompressedSize64)
				bytesRead, err := domainReader.Read(domainBackupBytes)
				if err != nil {
					logging.LogDebug("repo/dp/ExportDomain() - Error reading domain from backup archive.", err)
					return nil, err
				}

				if uint64(bytesRead) != file.UncompressedSize64 {
					logging.LogDebug("repo/dp/ExportDomain() - Wrong number of bytes read for domain from backup archive.", err)
					return nil, errs.Errorf("Error reading domain from DataPower backup archive.")
				}

				return domainBackupBytes, err
			}
		}

		return nil, errs.Errorf("Export failed, domain '%s' not found in DataPower backup.", domainName)
	default:
		return nil, errs.Errorf("DataPower management interface %s not supported.", r.dataPowerAppliance.DpManagmentInterface())
	}
}

// SecureBackupAppliance creates secure backup of DataPower appliance using
// given Certificate object certName on the given exportDestPath and returns
// error in case of error or nil for success.
func (r *dpRepo) SecureBackupAppliance(applianceConfigName, certName, exportDestPath string) error {
	logging.LogDebugf("repo/dp/SecureBackupAppliance('%s', '%s', '%s')",
		applianceConfigName, certName, exportDestPath)

	// 0. Prepare DataPower connection configuration.
	oldDataPowerAppliance := r.dataPowerAppliance
	r.dataPowerAppliance = dpApplicance{name: applianceConfigName,
		DataPowerAppliance: config.Conf.DataPowerAppliances[applianceConfigName]}
	clearCurrentConfig := func() {
		r.dataPowerAppliance = oldDataPowerAppliance
	}
	defer clearCurrentConfig()
	if r.dataPowerAppliance.Password == "" {
		r.dataPowerAppliance.SetDpPlaintextPassword(config.DpTransientPasswordMap[applianceConfigName])
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// Don't know how to create secure backup using REST.
		return errs.Errorf("DataPower management interface %s not supported for secure backup of the appliance.",
			r.dataPowerAppliance.DpManagmentInterface())
	case config.DpInterfaceSoma:
		secureBackupRequestSoma := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request>
			<man:do-action>
				<SecureBackup>
					<cert>%s</cert>
					<destination>%s</destination>
					<include-iscsi>off</include-iscsi>
					<include-raid>off</include-raid>
				</SecureBackup>
			</man:do-action>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, certName, exportDestPath)
		secureBackupResponseSoma, err := r.soma(secureBackupRequestSoma)
		if err != nil {
			return err
		}
		result, err := parseSOMAFindOne(secureBackupResponseSoma, "//*[local-name()='response']/*[local-name()='result']")
		if err != nil {
			return err
		}
		logging.LogDebugf("repo/dp/SecureBackupAppliance(), result: '%v'", result)
		if result != "OK" {
			return errs.Errorf("DataPower secure backup error: '%v'", result)
		}
		return nil
	default:
		return errs.Errorf("DataPower management interface %s not supported.", r.dataPowerAppliance.DpManagmentInterface())
	}
}

// GetObjectDetails parses DataPower export to show service policy
// with all rules, matches & actions.
func (r *dpRepo) GetObjectDetails(domainName, objectClassName, objectName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetObjectDetails('%s', '%s', '%s')",
		domainName, objectClassName, objectName)
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// 1. Start export (send export request)
		exportRequestJSON := fmt.Sprintf(`{"Export":
		  {
		    "Format":"ZIP",
		    "UserComment":"Created by dpcmder.",
		    "AllFiles":"off",
		    "Persisted":"off",
		    "IncludeInternalFiles":"off",
				"Object":
		      [
		        {
		          "class":"%s",
		          "name":"%s",
		          "ref-objects":"on",
		          "ref-files":"off",
		          "include-debug":"off"
		        }
		      ]
		  }
		}`, objectClassName, objectName)
		locationURL, _, err := r.restPostForResult(
			"/mgmt/actionqueue/"+domainName,
			exportRequestJSON,
			"/Export/status",
			"Action request accepted.",
			"/_links/location/href")
		if err != nil {
			return nil, err
		}

		timeStart := time.Now()
		for {
			// 2. Check for current status of export request
			status, exportResponseJSON, err := r.restGetForOneResult(locationURL, "/status")
			logging.LogDebugf("repo/dp/GetObjectDetails() status: '%s'", status)
			if err != nil {
				return nil, err
			}

			switch status {
			case "started":
				if time.Since(timeStart) > 120*time.Second {
					logging.LogDebugf("repo/dp/GetObjectDetails() waiting for export since %v, giving up.\n last exportResponseJSON: '%s'", timeStart, exportResponseJSON)
					return nil, errs.Errorf("Export didn't finish since %v, giving up.", timeStart)
				}
				time.Sleep(1 * time.Second)
			case "completed":
				// 3. When export is completed get base64 result file from it
				logging.LogDebugf("repo/dp/GetObjectDetails() export fetched after %d.", time.Since(timeStart))
				fileB64, err := parseJSONFindOne(exportResponseJSON, "/result/file")
				if err != nil {
					return nil, err
				}
				fileBytes, err := base64.StdEncoding.DecodeString(fileB64)
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error decoding b64 encoded export file.", err)
					return nil, err
				}

				// 4. Extract export.xml from zip archive
				exportBytesReader := bytes.NewReader(fileBytes)
				exportZipReader, err := zip.NewReader(exportBytesReader, int64(len(fileBytes)))
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error unzipping export archive.", err)
					return nil, err
				}
				if len(exportZipReader.File) != 1 {
					logging.LogDebugf("repo/dp/GetObjectDetails() - Unexpected number of compressed files (%d).",
						len(exportZipReader.File))
					return nil, errs.Errorf("Unexpected number of compressed files (%d)", len(exportZipReader.File))
				}

				exportXMLFile := exportZipReader.File[0]
				exportXMLReader, err := exportXMLFile.Open()
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error opening export.xml from export archive for reading.", err)
					return nil, err
				}
				defer exportXMLReader.Close()

				exportXMLBytes := make([]byte, exportXMLFile.UncompressedSize64)
				bytesRead, err := io.ReadFull(exportXMLReader, exportXMLBytes)
				if err == io.EOF {
					err = nil
				}
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error reading export.xml from export archive.", err)
					return nil, err
				}

				if uint64(bytesRead) != exportXMLFile.UncompressedSize64 {
					logging.LogDebug("repo/dp/GetObjectDetails() - Wrong number of bytes read for export.xml from export archive.", err)
					return nil, errs.Errorf("Error reading export.xml from DataPower export archive.")
				}

				return getObjectDetailsFromExportXML(exportXMLBytes, objectClassName, objectName)
			default:
				return nil, errs.Errorf("Unexpected response from server ('%s').", status)
			}
		}
	case config.DpInterfaceSoma:
		// 1. Fetch export of domain
		//    Backup contains domain export zip + export info and dp-aux files
		exportRequestSoma := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
   <soapenv:Header/>
   <soapenv:Body>
      <man:request domain="%s">
         <man:do-export format="ZIP" all-files="false"
              persisted="false" deployment-policy="no-internal-files">
            <man:user-comment>do-export</man:user-comment>
            <man:object class="%s" name="%s"
                ref-objects="true" ref-files="false" include-debug="false"/>
         </man:do-export>
      </man:request>
   </soapenv:Body>
</soapenv:Envelope>`, domainName, objectClassName, objectName)
		exportResponseSoma, err := r.soma(exportRequestSoma)
		if err != nil {
			return nil, err
		}
		exportFileB64, err := parseSOMAFindOne(exportResponseSoma, "//*[local-name()='file']")
		if err != nil {
			return nil, err
		}

		exportBytes, err := base64.StdEncoding.DecodeString(exportFileB64)
		if err != nil {
			logging.LogDebug("repo/dp/GetObjectDetails() - Error decoding base64 file.", err)
			return nil, err
		}

		// 2. Extract just export.xml file
		exportBytesReader := bytes.NewReader(exportBytes)
		exportZipReader, err := zip.NewReader(exportBytesReader, int64(len(exportBytes)))
		if err != nil {
			logging.LogDebug("repo/dp/GetObjectDetails() - Error unzipping backup archive.", err)
			return nil, err
		}
		for idx, file := range exportZipReader.File {
			logging.LogDebugf("repo/dp/GetObjectDetails() - file[%d] : '%s'.", idx, file.Name)
			if file.Name == "export.xml" {
				exportXMLReader, err := file.Open()
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error opening export.xml from export archive for reading.", err)
					return nil, err
				}
				defer exportXMLReader.Close()

				exportXMLBytes := make([]byte, file.UncompressedSize64)
				bytesRead, err := io.ReadFull(exportXMLReader, exportXMLBytes)

				if err == io.EOF {
					err = nil
				}
				if err != nil {
					logging.LogDebug("repo/dp/GetObjectDetails() - Error reading export.xml from export archive.", err)
					return nil, err
				}

				if uint64(bytesRead) != file.UncompressedSize64 {
					logging.LogDebugf("repo/dp/GetObjectDetails() - Wrong number of bytes (%d/%d) read for export.xml from export archive, err: %v",
						bytesRead, file.UncompressedSize64, err)
					return nil, errs.Errorf("Error reading export.xml from DataPower export archive.")
				}

				return getObjectDetailsFromExportXML(exportXMLBytes, objectClassName, objectName)
			}
		}

		return nil, errs.Errorf("Export failed, domain '%s' not found in DataPower backup.", domainName)
	default:
		return nil, errs.Errorf("DataPower management interface %s not supported.", r.dataPowerAppliance.DpManagmentInterface())
	}
}

// getObjectRulesFromExportXML parses export.xml bytes and returns nicely
// formatted info about service (matches, rules, actions).
func getObjectDetailsFromExportXML(exportXMLBytes []byte, objectClassName, objectName string) ([]byte, error) {
	doc, err := xmlquery.Parse(bytes.NewReader(exportXMLBytes))
	if err != nil {
		logging.LogDebug("Error parsing DataPower export.xml.", err)
		return nil, err
	}

	// getReferencedObjectInfo returns class and name of referenced object.
	getReferencedObjectInfo := func(node *xmlquery.Node, elemName string) (string, string) {
		subNode := node.SelectElement(elemName)
		if subNode != nil {
			return subNode.SelectAttr("class"), subNode.InnerText()
		}
		return "", ""
	}

	// getSubnodeInnerText returns text inside XML node or "" if node doesn't exist.
	getSubnodeInnerText := func(node *xmlquery.Node, elemName string) string {
		_, name := getReferencedObjectInfo(node, elemName)
		return name
	}

	// createConfigQuery returns query used to search element in configuration
	// node. Used when we want to search for objects referenced from another object.
	createConfigQuery := func(subNode *xmlquery.Node) string {
		if subNode == nil {
			return "invalid-node-expression"
		}

		subNodeName := subNode.InnerText()
		subNodeClass := subNode.SelectAttr("class")
		configQuery := fmt.Sprintf("/%s[@name='%s']", subNodeClass, subNodeName)

		return configQuery
	}

	// getObjectDetailsForMatch returns details for given Matching object.
	getObjectDetailsForMatch := func(configNode *xmlquery.Node, matchClass, matchName string) ([]byte, error) {
		matchQuery := fmt.Sprintf("/%s[@name='%s']", matchClass, matchName)
		matchNode := xmlquery.FindOne(configNode, matchQuery)
		if matchNode == nil {
			logging.LogDebugf("Can't find '%s' match in export.xml", matchQuery)
			return nil, errs.Errorf("Can't find '%s' match in export.xml", matchQuery)
		}

		mrNodes := matchNode.SelectElements("MatchRules")
		mrTxt := ""
		for _, mrNode := range mrNodes {
			mrType := getSubnodeInnerText(mrNode, "Type")
			if mrTxt != "" {
				mrTxt = mrTxt + " "
			}
			mrTxt = mrTxt + mrType + "("
			switch mrType {
			case "url", "fullyqualifiedurl":
				mrTxt = mrTxt + getSubnodeInnerText(mrNode, "Url")
			case "http":
				mrTxt = mrTxt + getSubnodeInnerText(mrNode, "HttpTag") + ": " + getSubnodeInnerText(mrNode, "HttpValue")
			case "xpath":
				mrTxt = mrTxt + getSubnodeInnerText(mrNode, "XPATHExpression")
			case "errorcode":
				mrTxt = mrTxt + getSubnodeInnerText(mrNode, "errorcode")
			default:
				mrTxt = mrTxt + mrNode.InnerText()
			}
			mrTxt = mrTxt + ")"
		}

		matchTxt := fmt.Sprintf("  Match (%s): [%s]\n",
			matchNode.SelectAttr("name"), mrTxt)

		return []byte(matchTxt), nil
	}

	// getObjectDetailsForRule returns details for given Rule* object.
	getObjectDetailsForRule := func(configNode *xmlquery.Node, ruleClass, ruleName string) ([]byte, error) {
		ruleQuery := fmt.Sprintf("/%s[@name='%s']", ruleClass, ruleName)
		ruleNode := xmlquery.FindOne(configNode, ruleQuery)
		if ruleNode == nil {
			logging.LogDebugf("Can't find '%s' rule in export.xml", ruleQuery)
			return nil, errs.Errorf("Can't find '%s' rule in export.xml", ruleQuery)
		}

		ruleActionNodes := ruleNode.SelectElements("Actions")
		actionsTxt := ""
		for _, ruleActionNode := range ruleActionNodes {
			actionQuery := createConfigQuery(ruleActionNode)
			actionNode := xmlquery.FindOne(configNode, actionQuery)
			if actionNode == nil {
				logging.LogDebugf("Can't find '%s' action in export.xml", actionQuery)
				return nil, errs.Errorf("Can't find '%s' action in export.xml", actionQuery)
			}

			actionsTxt = fmt.Sprintf("%s    Action (%s -> %s -> %s): %s(%s%s)\n", actionsTxt,
				getSubnodeInnerText(actionNode, "Input"),
				actionNode.SelectAttr("name"),
				getSubnodeInnerText(actionNode, "Output"),
				getSubnodeInnerText(actionNode, "Type"),
				getSubnodeInnerText(actionNode, "Transform"),
				getSubnodeInnerText(actionNode, "GatewayScriptLocation"))
		}

		ruleTxt := fmt.Sprintf("  Rule (%s / %s):\n%s\n",
			ruleNode.SelectAttr("name"),
			getSubnodeInnerText(ruleNode, "Direction"),
			actionsTxt)

		return []byte(ruleTxt), nil
	}

	configQuery := "/datapower-configuration/configuration"
	configNode := xmlquery.FindOne(doc, configQuery)
	if configNode == nil {
		logging.LogDebugf("Can't find '%s' in export.xml", configQuery)
		return nil, errs.Errorf("Can't find '%s' in export.xml", configQuery)
	}

	// getObjectDetailsForPolicy returns details for given StylePolicy* object.
	getObjectDetailsForPolicy := func(configNode *xmlquery.Node, policyClass, policyName string) ([]byte, error) {
		policyQuery := fmt.Sprintf("/%s[@name='%s']", policyClass, policyName)
		policyNode := xmlquery.FindOne(configNode, policyQuery)
		if policyNode == nil {
			logging.LogDebugf("Can't find '%s' policy in configuration in export.xml", policyQuery)
			return nil, errs.Errorf("Can't find '%s' policy in configuration in export.xml", policyQuery)
		}

		policyMapNodes := policyNode.SelectElements("PolicyMaps")
		policyTxt := fmt.Sprintf("%s (%s)\n", policyClass, policyName)
		for _, policyMapNode := range policyMapNodes {
			// policyMatchNode := policyMapNode.SelectElement("Match")
			matchClass, matchName := getReferencedObjectInfo(policyMapNode, "Match")
			matchTxt, err := getObjectDetailsForMatch(configNode, matchClass, matchName)
			if err != nil {
				return nil, err
			}

			// policyRuleNode := policyMapNode.SelectElement("Rule")
			ruleClass, ruleName := getReferencedObjectInfo(policyMapNode, "Rule")
			ruleTxt, err := getObjectDetailsForRule(configNode, ruleClass, ruleName)
			if err != nil {
				return nil, err
			}

			policyTxt = fmt.Sprintf("%s%s%s", policyTxt, matchTxt, ruleTxt)
		}

		return []byte(policyTxt), nil
	}

	// TODO: check ConfigMQproxy ?
	switch objectClassName {
	case "B2BProfile",
		"MultiProtocolGateway",
		"WSGateway",
		"XMLFirewallService",
		"XSLProxyService",
		"WebAppFW",
		"WebTokenService":
		svcQuery := fmt.Sprintf("/%s[@name='%s']", objectClassName, objectName)
		svcNode := xmlquery.FindOne(configNode, svcQuery)
		if svcNode == nil {
			logging.LogDebugf("Can't find '%s' in configuration in export.xml", svcQuery)
			return nil, errs.Errorf("Can't find '%s' in configuration in export.xml", svcQuery)
		}

		var svcPolicyName string
		switch objectClassName {
		case "B2BProfile":
			svcPolicyName = "CustomStylePolicy"
		default:
			svcPolicyName = "StylePolicy"
		}
		svcPolicyNode := svcNode.SelectElement(svcPolicyName)
		styleQuery := createConfigQuery(svcPolicyNode)
		styleNode := xmlquery.FindOne(configNode, styleQuery)
		if styleNode == nil {
			logging.LogDebugf("Can't find '%s' policy in export.xml", styleQuery)
			return nil, nil
		}

		svcPolicyClass, svcPolicyName := getReferencedObjectInfo(svcNode, svcPolicyName)
		policyTxt, err := getObjectDetailsForPolicy(configNode, svcPolicyClass, svcPolicyName)
		if err != nil {
			return nil, err
		}

		svcTxt := fmt.Sprintf("%s (%s)\n%s", objectClassName, objectName, policyTxt)
		return []byte(svcTxt), nil

	case "WSStylePolicy",
		"StylePolicy":
		return getObjectDetailsForPolicy(configNode, objectClassName, objectName)

	case "Matching":
		return getObjectDetailsForMatch(configNode, objectClassName, objectName)

	case "StylePolicyRule",
		"WSStylePolicyRule",
		"RequestStylePolicyRule",
		"ResponseStylePolicyRule",
		"ErrorStylePolicyRule":
		return getObjectDetailsForRule(configNode, objectClassName, objectName)

	default:
		return nil, errs.Errorf("Unrecognized object class '%s'.", objectClassName)
	}
}

// GetObject fetches DataPower object configuration. If persisted flag is true
// fetch persisted object, otherwise fetch current object from memory.
func (r *dpRepo) GetObject(dpDomain, objectClass, objectName string, persisted bool) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetObject('%s', '%s', '%s', %t)",
		dpDomain, objectClass, objectName, persisted)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		if persisted {
			return nil, errs.Errorf("Can't get persisted object using REST managment.")
		}
		getObjectURL := fmt.Sprintf("/mgmt/config/%s/%s/%s",
			dpDomain, objectClass, objectName)
		objectJSON, err := r.restGet(getObjectURL)
		if err != nil {
			if respErr, ok := err.(errs.UnexpectedHTTPResponse); ok && respErr.StatusCode == 404 {
				return nil, nil
			}
			return nil, err
		}

		logging.LogDebugf("repo/dp/GetObject(), objectJSON: '%s'", objectJSON)
		cleanedJSON, err := cleanJSONObject(objectJSON)
		if err != nil {
			return nil, err
		}
		return []byte(cleanedJSON), nil
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-config class="%s" name="%s" persisted="%t"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`,
			dpDomain, objectClass, objectName, persisted)
		somaResponse, err := r.soma(somaRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}

		query := "//*[local-name()='response']/*[local-name()='config']/*"
		resultNode := xmlquery.FindOne(doc, query)
		if resultNode == nil {
			logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
			// return nil, errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
			return nil, nil
		}

		resultXML := resultNode.OutputXML(true)
		cleanedXML, err := cleanXML(resultXML)
		if err != nil {
			return nil, err
		}
		return []byte(cleanedXML), nil
	default:
		logging.LogDebug("repo/dp/GetObject(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// SetObject updates or creates DataPower object configuration.
func (r *dpRepo) SetObject(dpDomain, objectClass, objectName string, objectContent []byte, existingObject bool) error {
	logging.LogDebugf("repo/dp/SetObject('%s', '%s', '%s', .., %t)",
		dpDomain, objectClass, objectName, existingObject)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		var setObjectURL string
		var setObjectMethod string
		var err error
		if existingObject {
			setObjectURL = fmt.Sprintf("/mgmt/config/%s/%s/%s",
				dpDomain, objectClass, objectName)
			setObjectMethod = "PUT"
		} else {
			setObjectURL = fmt.Sprintf("/mgmt/config/%s/%s",
				dpDomain, objectClass)
			setObjectMethod = "POST"
		}
		resultJSON, err := r.rest(setObjectURL, setObjectMethod, string(objectContent))
		if err != nil {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), resultJSON: '%s'", resultJSON)
		errorMessage, err := parseJSONFindOne(resultJSON, "/error")
		if err != nil && err.Error() != "Unexpected JSON, can't find '/error'." {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), errorMessage: '%s'", errorMessage)
		successMessage, err := parseJSONFindOne(resultJSON, fmt.Sprintf("/%s", objectName))
		if err != nil {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), successMessage: '%s'", successMessage)
		return err
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:set-config>
%s
			</man:set-config>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, dpDomain, objectContent)
		logging.LogDebugf("repo/dp/SetObject(), somaRequest: '%s'", somaRequest)
		somaResponse, err := r.soma(somaRequest)
		if err != nil {
			return err
		}

		logging.LogDebugf("repo/dp/SetObject(), somaResponse: '%s'", somaResponse)
		resultMsg, err := parseSOMAFindOne(somaResponse, "//*[local-name()='response']/*[local-name()='result']")
		if err != nil {
			return err
		}
		if resultMsg != "OK" {
			return errs.Errorf("Unexpected result of SOMA update: '%s'.", resultMsg)
		}

		return nil
	default:
		logging.LogDebug("repo/dp/SetObject(), using neither REST neither SOMA.")
		return errs.Error("DataPower management interface not set.")
	}
}

// RenameObject changes name in DataPower object configuration (JSON or XML).
func (r *dpRepo) RenameObject(dpObject []byte, objectName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/RenameObject(.., '%s')", objectName)
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// Replace name field ("name": "example-Firewall5",)
		r := regexp.MustCompile(`("name": *")([^"]+)(",)`)
		dpObjectRenamed := r.ReplaceAllString(string(dpObject), "${1}"+objectName+"${3}")
		logging.LogTracef("repo/dp/RenameObject(), rest, dpObject: '%s'", dpObject)
		logging.LogTracef("repo/dp/RenameObject(), rest, renamed: '%s'", dpObjectRenamed)
		return []byte(dpObjectRenamed), nil
	case config.DpInterfaceSoma:
		// Replace name attribute (<XMLFirewallService name="example-Firewall5">)
		r := regexp.MustCompile(`(name=")([^"]*)(")`)
		dpObjectRenamed := r.ReplaceAllString(string(dpObject), "${1}"+objectName+"${3}")
		logging.LogTracef("repo/dp/RenameObject(), soma, dpObject: '%s'", dpObject)
		logging.LogTracef("repo/dp/RenameObject(), soma, renamed: '%s'", dpObjectRenamed)
		return []byte(dpObjectRenamed), nil
	default:
		logging.LogDebug("repo/dp/RenameObject(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// GetStatus fetches DataPower status info.
func (r *dpRepo) GetStatus(dpDomain, statusClass string, statusIdx int) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetStatus('%s', '%s', %d)",
		dpDomain, statusClass, statusIdx)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		getStatusesURL := fmt.Sprintf("/mgmt/status/%s/%s",
			dpDomain, statusClass)
		statusesRespJSON, err := r.restGet(getStatusesURL)
		if err != nil {
			if respErr, ok := err.(errs.UnexpectedHTTPResponse); ok && respErr.StatusCode == 404 {
				return nil, nil
			}
			return nil, err
		}
		logging.LogDebugf("repo/dp/GetStatus(), statusesRespJSON: '%s'", statusesRespJSON)

		jqQueryStatuses := fmt.Sprintf(".%s", statusClass)
		jqOpStatuses, err := jq.Parse(jqQueryStatuses)
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatus(), error parsing query '%s', %v",
				jqOpStatuses, err)
			return nil, err
		}
		statusesJSON, err := jqOpStatuses.Apply([]byte(statusesRespJSON))
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatus(), error applying query '%s', %v",
				jqQueryStatuses, err)
			return nil, err
		}
		logging.LogDebugf("repo/dp/GetStatus(), statusesJSON: '%s'", statusesJSON)

		var jqQuery string
		firstChar := string(statusesJSON[0:1])
		switch firstChar {
		case "[":
			jqQuery = fmt.Sprintf(".[%d]", statusIdx)
		case "{":
			jqQuery = "."
		default:
			return nil, errs.Errorf("Unexpected result for jqQuery '%s' on '%s'.",
				jqQuery, statusesJSON)
		}

		jqOp, err := jq.Parse(jqQuery)
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatus(), error parsing query '%s', %v",
				jqQuery, err)
			return nil, err
		}
		statusJSON, err := jqOp.Apply([]byte(statusesJSON))
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatus(), error applying query '%s', %v",
				jqQuery, err)
			return nil, err
		}
		logging.LogDebugf("repo/dp/GetStatus(), statusJSON: '%s'", statusJSON)

		formattedJSON, err := cleanJSONObject(string(statusJSON))
		if err != nil {
			return nil, err
		}
		return []byte(formattedJSON), nil
	case config.DpInterfaceSoma:
		somaStatusRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-status class="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`,
			dpDomain, statusClass)
		somaResponse, err := r.soma(somaStatusRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}

		query := fmt.Sprintf(
			"//*[local-name()='response']/*[local-name()='status']/*")
		resultNodes := xmlquery.Find(doc, query)
		resultNode := resultNodes[statusIdx]
		if resultNode == nil {
			logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
			return nil, errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
		}

		resultXML := resultNode.OutputXML(true)
		formattedXML, err := cleanXML(resultXML)
		if err != nil {
			return nil, err
		}
		return []byte(formattedXML), nil
	default:
		logging.LogDebug("repo/dp/GetStatus(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// GetStatuses fetches DataPower status info for all statuses in class.
func (r *dpRepo) GetStatuses(dpDomain, statusClass string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetStatuses('%s', '%s')", dpDomain, statusClass)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		getStatusesURL := fmt.Sprintf("/mgmt/status/%s/%s",
			dpDomain, statusClass)
		statusesRespJSON, err := r.restGet(getStatusesURL)
		if err != nil {
			if respErr, ok := err.(errs.UnexpectedHTTPResponse); ok && respErr.StatusCode == 404 {
				return nil, nil
			}
			return nil, err
		}
		logging.LogDebugf("repo/dp/GetStatuses(), statusesRespJSON: '%s'", statusesRespJSON)

		jqQueryStatuses := fmt.Sprintf(".%s", statusClass)
		jqOpStatuses, err := jq.Parse(jqQueryStatuses)
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatuses(), error parsing query '%s', %v",
				jqOpStatuses, err)
			return nil, err
		}
		statusesJSON, err := jqOpStatuses.Apply([]byte(statusesRespJSON))
		if err != nil {
			logging.LogDebugf("repo/dp/GetStatuses(), error applying query '%s', %v",
				jqQueryStatuses, err)
			return nil, err
		}
		logging.LogDebugf("repo/dp/GetStatuses(), statusesJSON: '%s'", statusesJSON)

		formattedJSON, err := cleanJSONObject(string(statusesJSON))
		if err != nil {
			return nil, err
		}
		return []byte(formattedJSON), nil
	case config.DpInterfaceSoma:
		somaStatusRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-status class="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`,
			dpDomain, statusClass)
		somaResponse, err := r.soma(somaStatusRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}

		query := fmt.Sprintf(
			"//*[local-name()='response']/*[local-name()='status']")
		resultNode := xmlquery.FindOne(doc, query)
		if resultNode == nil {
			logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
			return nil, errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
		}

		resultXML := resultNode.OutputXML(true)
		formattedXML, err := cleanXML(resultXML)
		if err != nil {
			return nil, err
		}
		return []byte(formattedXML), nil
	default:
		logging.LogDebug("repo/dp/GetStatuses(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// SaveConfiguration saves current DataPower configuration.
func (r *dpRepo) SaveConfiguration(itemConfig *model.ItemConfig) error {
	logging.LogDebugf("repo/dp/SaveConfiguration(%v)", itemConfig)
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		saveConfigRequestJSON := `{"SaveConfig":"0"}`
		resultText, _, err := r.restPostForResult(
			"/mgmt/actionqueue/"+itemConfig.DpDomain,
			saveConfigRequestJSON,
			"/SaveConfig",
			"Operation completed.",
			"/SaveConfig")
		if err != nil {
			return err
		}

		logging.LogDebugf("repo/dp/SaveConfiguration(), resultText: '%s'", resultText)

		return nil
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:do-action><SaveConfig/></man:do-action>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, itemConfig.DpDomain)
		logging.LogDebugf("repo/dp/SaveConfiguration(), somaRequest: '%s'", somaRequest)
		somaResponse, err := r.soma(somaRequest)
		if err != nil {
			return err
		}

		logging.LogDebugf("repo/dp/SaveConfiguration(), somaResponse: '%s'", somaResponse)
		resultMsg, err := parseSOMAFindOne(somaResponse, "//*[local-name()='response']/*[local-name()='result']")
		if err != nil {
			return err
		}
		resultMsg = strings.TrimSpace(resultMsg)
		if resultMsg != "OK" {
			return errs.Errorf("Unexpected result of SOMA update: '%s'.", resultMsg)
		}

		return nil
	default:
		logging.LogDebug("repo/dp/SaveConfiguration(), using neither REST neither SOMA.")
		return errs.Error("DataPower management interface not set.")
	}
}

// CreateDomain creates new domain on DataPower appliance.
func (r *dpRepo) CreateDomain(domainName string) error {
	logging.LogDebugf("repo/dp/CreateDomain('%s')", domainName)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		return errs.Errorf("Can't create domain using REST management interface.")
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request>
			<man:set-config>
				<Domain name="%s">
					<NeighborDomain class="Domain">default</NeighborDomain>
				</Domain>
			</man:set-config>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, domainName)
		logging.LogDebugf("repo/dp/CreateDomain(), somaRequest: '%s'", somaRequest)
		somaResponse, err := r.soma(somaRequest)
		if err != nil {
			return err
		}

		logging.LogDebugf("repo/dp/CreateDomain(), somaResponse: '%s'", somaResponse)
		resultMsg, err := parseSOMAFindOne(somaResponse, "//*[local-name()='response']/*[local-name()='result']")
		if err != nil {
			return err
		}
		if resultMsg != "OK" {
			return errs.Errorf("Unexpected result of SOMA update: '%s'.", resultMsg)
		}

		return nil
	default:
		logging.LogDebug("repo/dp/CreateDomain(), using neither REST neither SOMA.")
		return errs.Error("DataPower management interface not set.")
	}
}

// ParseObjectClassAndName parses bytes with XML/JSON definition of object
// (XML/JSON should be used depending on REST/SOMA interface used).
func (r *dpRepo) ParseObjectClassAndName(objectBytes []byte) (objectClass, objectName string, err error) {
	logging.LogDebugf("repo/dp/ParseObjectClassAndName('%s')", objectBytes)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		doc, err := jsonquery.Parse(bytes.NewReader(objectBytes))
		if err != nil {
			logging.LogDebug("Error parsing JSON.", err)
			return "", "", err
		}

		rootNode := jsonquery.FindOne(doc, "/*")
		if rootNode == nil {
			logging.LogDebugf("Can't find class name in JSON object configuration:\n'%s'", objectBytes)
			return "", "", errs.Error("Unexpected XML, can't find class name.")
		}
		nameNode := rootNode.SelectElement("name")
		if nameNode == nil {
			logging.LogDebugf("Can't find object name in JSON object configuration:\n'%s'", objectBytes)
			return "", "", errs.Error("Unexpected XML, can't find object name.")
		}
		className := rootNode.Data
		objectName := nameNode.InnerText()
		logging.LogDebugf("repo/dp/ParseObjectClassAndName(), className: '%s', objectName: '%s'", className, objectName)

		return className, objectName, nil
	case config.DpInterfaceSoma:
		doc, err := xmlquery.Parse(bytes.NewReader(objectBytes))
		if err != nil {
			logging.LogDebug("Error parsing XML.", err)
			return "", "", err
		}

		rootNode := xmlquery.FindOne(doc, "/*")
		if rootNode == nil {
			logging.LogDebugf("Can't find class name in XML object configuration:\n'%s'", objectBytes)
			return "", "", errs.Error("Unexpected XML, can't find class name.")
		}

		className := rootNode.Data
		objectName := rootNode.SelectAttr("name")
		return className, objectName, nil
	default:
		logging.LogDebug("repo/dp/CreateDomain(), using neither REST neither SOMA.")
		return "", "", errs.Error("DataPower management interface not set.")
	}
}

// GetItemInfo returns information about given item.
func (r *dpRepo) GetItemInfo(itemConfig *model.ItemConfig) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetItemInfo(%v)", itemConfig)

	var itemInfo []byte
	var err error
	switch itemConfig.Type {
	case model.ItemDpObject:
		itemInfo, err = json.Marshal(itemConfig.DpObjectState)
		if err != nil {
			return nil, err
		}

		var prettyJSON bytes.Buffer
		json.Indent(&prettyJSON, itemInfo, "", "  ")
		itemInfo = prettyJSON.Bytes()
	default:
		return nil, errs.Errorf("No additional info available for item '%s'.",
			itemConfig.Name)
	}

	return itemInfo, err
}

func (r *dpRepo) FlushCache(
	domainName, statusClass, statusName string, itemType model.ItemType) (bool, error) {
	logging.LogDebugf("repo/dp/FlushCache('%s', '%s', '%s' (%s))",
		domainName, statusClass, statusName, itemType)

	switch itemType {
	case model.ItemDpStatusClass:
		switch r.dataPowerAppliance.DpManagmentInterface() {
		case config.DpInterfaceRest:
			switch statusClass {
			case "StylesheetCachingSummary", "DocumentCachingSummary":
				getStatusesURL := fmt.Sprintf("/mgmt/status/%s/%s", domainName, statusClass)
				getStatusesQuery := fmt.Sprintf("/%s//XMLManager/value", statusClass)
				statusNames, _, err :=
					r.restGetForListResult(getStatusesURL, getStatusesQuery)
				if err != nil {
					return false, err
				}
				for _, statusName := range statusNames {
					res, err := r.FlushCache(
						domainName, statusClass, statusName, model.ItemDpStatus)
					if err != nil || !res {
						return res, err
					}
				}
				return true, nil
			default:
				return false, errs.Errorf("Don't know to flush cache for '%s'.", statusClass)
			}
		case config.DpInterfaceSoma:
			switch statusClass {
			case "StylesheetCachingSummary", "DocumentCachingSummary":
				somaStatusRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
			xmlns:man="http://www.datapower.com/schemas/management">
			<soapenv:Body>
				<man:request domain="%s">
					<man:get-status class="%s"/>
				</man:request>
			</soapenv:Body>
		</soapenv:Envelope>`, domainName, statusClass)
				doc, err := r.somaGetDoc(somaStatusRequest)
				if err != nil {
					return false, err
				}

				query :=
					fmt.Sprintf("//*[local-name()='response']/*[local-name()='status']/*[local-name()='%s']/*[local-name()='XMLManager']",
						statusClass)
				nodes := xmlquery.Find(doc, query)
				if len(nodes) == 0 {
					return false, errs.Errorf("No status for class '%s' found.", statusClass)
				}
				for _, node := range nodes {
					statusName := node.InnerText()
					res, err := r.FlushCache(
						domainName, statusClass, statusName, model.ItemDpStatus)
					if err != nil || !res {
						return res, err
					}
				}
				return true, nil
			default:
				return false, errs.Errorf("Don't know to flush cache for '%s'.", statusClass)
			}
		default:
			logging.LogDebug("repo/dp/FlushCache(), using neither REST neither SOMA.")
			return false, errs.Error("DataPower management interface not set.")
		}
	case model.ItemDpStatus:
		var flushCacheOp string
		switch statusClass {
		case "StylesheetCachingSummary":
			flushCacheOp = "FlushStylesheetCache"
		case "DocumentCachingSummary":
			flushCacheOp = "FlushDocumentCache"
		default:
			return false, errs.Errorf("Don't know to flush cache for '%s'.", statusClass)
		}

		switch r.dataPowerAppliance.DpManagmentInterface() {
		case config.DpInterfaceRest:
			restActionPath := fmt.Sprintf("/mgmt/actionqueue/%s", domainName)
			logging.LogDebugf("repo/dp/FlushCache(), restActionPath: '%s'", restActionPath)
			flushRequestJSON :=
				fmt.Sprintf(`{"%s":{"XMLManager":"%s"}}`, flushCacheOp, statusName)

			jsonResponseString, err := r.rest(restActionPath, "POST", flushRequestJSON)
			if err != nil {
				return false, err
			}
			logging.LogDebugf("jsonResponseString: '%s'", jsonResponseString)
			resultMsg, err := parseJSONFindOne(jsonResponseString,
				fmt.Sprintf("/%s", flushCacheOp))
			if err != nil {
				return false, err
			}
			if resultMsg == "Operation completed." {
				return true, nil
			}
			return false, errs.Errorf("Error flushing cache '%s': '%s'",
				statusName, resultMsg)

		case config.DpInterfaceSoma:
			somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:man="http://www.datapower.com/schemas/management">
   <soapenv:Body>
      <man:request domain="%s">
         <man:do-action>
            <%s>
               <XMLManager>%s</XMLManager>
            </%s>
         </man:do-action>
      </man:request>
   </soapenv:Body>
</soapenv:Envelope>`,
				domainName, flushCacheOp, statusName, flushCacheOp)
			somaResponse, err := r.soma(somaRequest)
			if err != nil {
				return false, err
			}
			resultMsg, err := parseSOMAFindOne(somaResponse, "//*[local-name()='response']/*[local-name()='result']")
			if err != nil {
				logging.LogDebug("Error parsing response SOAP.", err)
				return false, err
			}
			if resultMsg == "OK" {
				return true, nil
			}
			return false, errs.Errorf("Error flushing cache '%s': '%s'",
				statusName, resultMsg)
		default:
			logging.LogDebug("repo/dp/FlushCache(), using neither REST neither SOMA.")
			return false, errs.Error("DataPower management interface not set.")
		}
	default:
		logging.LogDebugf(
			"repo/dp/FlushCache(), don't know how to flush cache for item type %s.",
			itemType)
		return false, errs.Errorf("Don't know how to flush cache for item type %s.",
			itemType.UserFriendlyString())
	}
}

// GetManagementInterface returns current DataPower management interface used.
func (r *dpRepo) GetManagementInterface() string {
	return r.dataPowerAppliance.DpManagmentInterface()
}

// listAppliances returns ItemList of DataPower appliance Items from configuration.
func listAppliances() (model.ItemList, error) {
	appliances := config.Conf.DataPowerAppliances
	logging.LogDebugf("repo/dp/listAppliances(), appliances: %v", appliances)

	appliancesConfig := model.ItemConfig{Type: model.ItemNone}
	items := make(model.ItemList, len(appliances))
	idx := 0
	for name, config := range appliances {
		itemConfig := model.ItemConfig{Type: model.ItemDpConfiguration,
			Name:        name,
			DpAppliance: name,
			DpDomain:    config.Domain,
			Parent:      &appliancesConfig}
		items[idx] = model.Item{Name: name, Config: &itemConfig}
		idx++
	}

	sort.Sort(items)
	logging.LogDebugf("repo/dp/listAppliances(), items: %v", items)

	var err error
	if len(items) == 0 {
		err = errs.Error("No appliances found, have to configure dpcmder with command line params first.")
	}

	return items, err
}

// listDomains loads DataPower domains from current DataPower.
func (r *dpRepo) listDomains(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listDomains('%s')", selectedItemConfig)
	domains, err := r.fetchDpDomains()
	if err != nil {
		return nil, err
	}
	logging.LogDebugf("repo/dp/listDomains('%s'), domains: %v", selectedItemConfig, domains)

	items := make(model.ItemList, len(domains)+1)
	items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

	for idx, domain := range domains {
		itemConfig := model.ItemConfig{Type: model.ItemDpDomain,
			Name:        domain.name,
			DpAppliance: selectedItemConfig.DpAppliance,
			DpDomain:    domain.name,
			Parent:      selectedItemConfig}
		modified := ""
		opState := ""
		if domain.saveNeeded {
			modified = "*"
		}
		if domain.down {
			opState = "down"
		}
		items[idx+1] = model.Item{Name: domain.name, Modified: modified, Size: opState, Config: &itemConfig}
	}

	sort.Sort(items)

	return items, nil
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *dpRepo) listFilestores(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listFilestores('%s')", selectedItemConfig)
	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		jsonString, err := r.restGet("/mgmt/filestore/" + selectedItemConfig.DpDomain)
		if err != nil {
			return nil, err
		}
		// println("jsonString: " + jsonString)

		// .filestore.location[]?.name
		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogDebug("Error parsing response JSON.", err)
			return nil, err
		}
		filestoreNameNodes := jsonquery.Find(doc, "/filestore/location/*/name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore,
				Name:        filestoreName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: filestoreName,
				Path:        filestoreName,
				Parent:      selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items, nil
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:get-filestore layout-only="true" no-subdirectories="true"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, selectedItemConfig.DpDomain)
		dpFilestoresXML, err := r.soma(somaRequest)
		if err != nil {
			return nil, err
		}

		filestoreNames, err := parseSOMAFindList(dpFilestoresXML, "//*[local-name()='location']/@name")
		if err != nil {
			return nil, err
		}

		items := make(model.ItemList, len(filestoreNames)+1)
		items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

		for idx, filestoreName := range filestoreNames {
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore,
				Name:        filestoreName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: filestoreName,
				Path:        filestoreName,
				Parent:      selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items, nil
	default:
		logging.LogDebug("repo/dp/listFilestores(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// listDpDir loads DataPower directory (local:, local:///test,..).
func (r *dpRepo) listDpDir(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listDpDir('%s')", selectedItemConfig)
	parentDir := model.Item{Name: "..", Config: selectedItemConfig.Parent}
	filesDirs, err := r.listFiles(selectedItemConfig)
	if err != nil {
		return nil, err
	}

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, filesDirs...)

	return itemsWithParentDir, nil
}

func (r *dpRepo) fetchFilestoreIfNeeded(dpDomain, dpFilestoreLocation string, forceReload bool) error {
	if r.dataPowerAppliance.SomaUrl != "" {
		// If we open filestore or open file but want to reload - refresh current filestore XML cache.
		if forceReload || r.invalidateCache || r.dpFilestoreXmls[dpFilestoreLocation] == "" {
			somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body>
		<man:request xmlns:man="http://www.datapower.com/schemas/management" domain="%s">
			<man:get-filestore layout-only="false" no-subdirectories="false" location="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, dpDomain, dpFilestoreLocation)
			var err error
			r.dpFilestoreXmls[dpFilestoreLocation], err = r.soma(somaRequest)
			if err != nil {
				return err
			}
			r.invalidateCache = false
		}
	}
	return nil
}

func (r *dpRepo) listFiles(selectedItemConfig *model.ItemConfig) ([]model.Item, error) {
	logging.LogDebugf("repo/dp/listFiles('%s')", selectedItemConfig)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		items := make(model.ItemList, 0)
		currRestDirPath := strings.Replace(selectedItemConfig.Path, ":", "", 1)
		jsonString, err := r.restGet("/mgmt/filestore/" + selectedItemConfig.DpDomain + "/" + currRestDirPath)
		if err != nil {
			return nil, err
		}
		// println("jsonString: " + jsonString)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogDebug("Error parsing response JSON.", err)
			return nil, err
		}

		// "//" - work-around - for one directory we get JSON object, for multiple directories we get JSON array
		dirNodes := jsonquery.Find(doc, "/filestore/location/directory//name/..")
		for _, n := range dirNodes {
			dirDpPath := n.SelectElement("name").InnerText()
			_, dirName := splitOnLast(dirDpPath, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				Name:        dirName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: selectedItemConfig.DpFilestore,
				Path:        dirDpPath,
				Parent:      selectedItemConfig}
			item := model.Item{Name: dirName, Config: &itemConfig}
			items = append(items, item)
		}

		// "//" - work-around - for one file we get JSON object, for multiple files we get JSON array
		fileNodes := jsonquery.Find(doc, "/filestore/location/file//name/..")
		for _, n := range fileNodes {
			fileName := n.SelectElement("name").InnerText()
			fileSize := n.SelectElement("size").InnerText()
			fileModified := n.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				Name:        fileName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: selectedItemConfig.DpFilestore,
				Path:        paths.GetDpPath(selectedItemConfig.Path, fileName),
				Parent:      selectedItemConfig}
			item := model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
			items = append(items, item)
		}

		sort.Sort(items)
		return items, nil
	case config.DpInterfaceSoma:
		dpFilestoreLocation, _ := splitOnFirst(selectedItemConfig.Path, "/")
		dpFilestoreIsRoot := !strings.Contains(selectedItemConfig.Path, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node

		// If we open filestore or open file but want to reload - refresh current filestore XML cache.
		err := r.fetchFilestoreIfNeeded(selectedItemConfig.DpDomain, dpFilestoreLocation, dpFilestoreIsRoot)
		if err != nil {
			logging.LogDebug("Error parsing response SOMA.", err)
			return nil, err
		}

		if dpFilestoreIsRoot {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXmls[dpFilestoreLocation]))
			if err != nil {
				logging.LogDebug("Error parsing response SOMA.", err)
				return nil, err
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file")
			// println(dpFilestoreLocation)
		} else {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXmls[dpFilestoreLocation]))
			if err != nil {
				logging.LogDebug("Error parsing response SOAP.", err)
				return nil, err
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItemConfig.Path+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItemConfig.Path+"']/file")
		}

		dirNum := len(dpDirNodes)
		fileNum := len(dpFileNodes)
		items := make(model.ItemList, dirNum+fileNum)
		for idx, node := range dpDirNodes {
			// "local:"
			dirFullName := node.SelectAttr("name")
			_, dirName := splitOnLast(dirFullName, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				Name:        dirName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: selectedItemConfig.DpFilestore,
				Path:        dirFullName,
				Parent:      selectedItemConfig}
			// Path: selectedItemConfig.Path
			items[idx] = model.Item{Name: dirName, Config: &itemConfig}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				Name:        fileName,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				DpFilestore: selectedItemConfig.DpFilestore,
				Path:        selectedItemConfig.Path + "/" + fileName,
				Parent:      selectedItemConfig}
			// selectedItemConfig.Path
			items[idx+dirNum] = model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
		}

		sort.Sort(items)
		return items, nil
	default:
		logging.LogDebug("repo/dp/listFiles(), using neither REST neither SOMA.")
		return model.ItemList{}, errs.Error("DataPower management interface not set.")
	}
}

// listObjectClasses lists all object classes used in current DataPower domain.
func (r *dpRepo) listObjectClasses(currentView *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listObjectClasses(%v)", currentView)

	if currentView.DpAppliance == "" {
		return nil, errs.Error("Can't get object class list if DataPower appliance is not selected.")
	}

	if currentView.DpDomain == "" {
		return nil, errs.Error("Can't get object class list if DataPower domain is not selected.")
	}

	var err error
	var classNamesAndStatusesWithDuplicates [][]string
	classNameMap := make(map[string]int)
	classNameModifiedMap := make(map[string]bool)
	classNames := make([]string, 0)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		listObjectStatusesURL := fmt.Sprintf("/mgmt/status/%s/ObjectStatus", currentView.DpDomain)
		classNamesAndStatusesWithDuplicates, _, err =
			r.restGetForListsResult(listObjectStatusesURL,
				"/ObjectStatus//Class", "/ObjectStatus//ConfigState")

	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
	  <man:request domain="%s">
	    <man:get-status class="ObjectStatus"/>
	  </man:request>
	</soapenv:Body>
</soapenv:Envelope>`, currentView.DpDomain)
		var somaResponse string
		somaResponse, err = r.soma(somaRequest)
		if err != nil {
			return nil, err
		}

		classNamesAndStatusesWithDuplicates, err = parseSOMAFindLists(somaResponse,
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/Class",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/ConfigState")

	default:
		r.DpViewMode = model.DpFilestoreMode
		logging.LogDebug("repo/dp/listObjectClasses(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}

	if err != nil {
		return nil, err
	}

	for idx, className := range classNamesAndStatusesWithDuplicates[0] {
		if _, oldName := classNameMap[className]; !oldName {
			classNames = append(classNames, className)
		}
		if classNamesAndStatusesWithDuplicates[1][idx] != objectStatusSaved &&
			classNamesAndStatusesWithDuplicates[1][idx] != objectStatusExternal {
			classNameModifiedMap[className] = true
		}
		classNameMap[className]++
	}

	logging.LogDebugf("repo/dp/listObjectClasses(), classNames: %v", classNames)

	items := make(model.ItemList, len(classNames))
	for idx, className := range classNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpObjectClass,
			Name:        className,
			DpAppliance: currentView.DpAppliance,
			DpDomain:    currentView.DpDomain,
			Path:        className,
			Parent:      currentView}
		modified := ""
		if classNameModifiedMap[className] {
			modified = "*"
		}
		item := model.Item{Name: className,
			Size:     fmt.Sprintf("%d", classNameMap[className]),
			Modified: modified,
			Config:   &itemConfig}
		items[idx] = item
	}

	sort.Sort(items)

	logging.LogDebugf("repo/dp/listObjectClasses(), items: %v", items)
	return items, nil
}

// listObjects lists all objects of selected class in current DataPower domain.
func (r *dpRepo) listObjects(itemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listObjects(%v)", itemConfig)

	switch itemConfig.Type {
	case model.ItemDpObjectClass:
	default:
		return nil, errs.Error("Can't get object list if no object class is known.")
	}

	objectClassName := itemConfig.Path

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// To get object status we have to fetch all object statuses and merge this
		// info with each object configuration info because:
		// Sometimes, response for ObjectStatus has invalid Name element - different
		// from name attribute in response for configuration of same object.
		// It seems it is wrong for "singleton" objects in default domain, for
		// example: WebGUI (Name (status): "web-mgmt", name (config): "WebGUI-Settings").
		listObjectStatusesURL := fmt.Sprintf("/mgmt/status/%s/ObjectStatus", itemConfig.DpDomain)
		objectNamesAndStatuses, _, err :=
			r.restGetForListsResult(listObjectStatusesURL,
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../Name", objectClassName),
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../ConfigState", objectClassName),
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../OpState", objectClassName),
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../AdminState", objectClassName),
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../EventCode", objectClassName),
				fmt.Sprintf("/ObjectStatus//Class[text()='%s']/../ErrorCode", objectClassName))
		if err != nil {
			return nil, err
		}

		listObjectsURL := fmt.Sprintf("/mgmt/config/%s/%s", itemConfig.DpDomain, objectClassName)
		objectNameQuery := fmt.Sprintf("/%s//name", objectClassName)
		objectNames, _, err := r.restGetForListResult(listObjectsURL, objectNameQuery)
		if err != nil {
			return nil, err
		}

		logging.LogDebugf("repo/dp/listObjects(), objectNames: %v", objectNames)
		parentDir := model.Item{Name: "..", Config: itemConfig.Parent}

		items := make(model.ItemList, len(objectNames))
		items = append(items, parentDir)

		for idx, objectNameFromStatus := range objectNamesAndStatuses[0] {
			// For "singleton" intrinsic objects in default domain we can't use name
			// from ObjectStatus.
			objectName := objectNameFromStatus
			if len(objectNames) == 1 {
				objectName = objectNames[0]
			}
			modified := ""
			if objectNamesAndStatuses[1][idx] != objectStatusSaved {
				modified = objectNamesAndStatuses[1][idx]
			}
			opState := ""
			if objectNamesAndStatuses[2][idx] != "up" {
				opState = objectNamesAndStatuses[2][idx]
			}

			itemConfig := model.ItemConfig{Type: model.ItemDpObject,
				Name:        objectName,
				DpAppliance: itemConfig.DpAppliance,
				DpDomain:    itemConfig.DpDomain,
				Path:        objectClassName,
				Parent:      itemConfig}
			itemConfig.DpObjectState.ConfigState = objectNamesAndStatuses[1][idx]
			itemConfig.DpObjectState.OpState = objectNamesAndStatuses[2][idx]
			itemConfig.DpObjectState.AdminState = objectNamesAndStatuses[3][idx]
			itemConfig.DpObjectState.EventCode = objectNamesAndStatuses[4][idx]
			itemConfig.DpObjectState.ErrorCode = objectNamesAndStatuses[5][idx]

			item := model.Item{Name: objectName, Modified: modified, Size: opState, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/listObjects(), items: %v", items)
		return items, nil
	case config.DpInterfaceSoma:
		// Sometimes, response for ObjectStatus has invalid Name element - different
		// from name attribute in response for configuration of same object.
		// It seems it is wrong for "singleton" objects in default domain, for
		// example: WebGUI (Name (status): "web-mgmt", name (config): "WebGUI-Settings").
		// Maybe these are cases where SOMA response has attribute intrinsic="true".
		somaConfigRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-config class="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, itemConfig.DpDomain, objectClassName)
		somaStatusRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-status class="ObjectStatus" object-class="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, itemConfig.DpDomain, objectClassName)
		somaConfigResponse, err := r.soma(somaConfigRequest)
		if err != nil {
			return nil, err
		}
		somaStatusResponse, err := r.soma(somaStatusRequest)
		if err != nil {
			return nil, err
		}

		objectNames, err := parseSOMAFindList(somaConfigResponse,
			"//*[local-name()='response']/*[local-name()='config']/*/@name")
		if err != nil {
			return nil, err
		}
		logging.LogDebugf("repo/dp/listObjects(), objectNames: %v", objectNames)

		objectNamesAndStatuses, err := parseSOMAFindLists(somaStatusResponse,
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/Name",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/ConfigState",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/OpState",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/AdminState",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/EventCode",
			"//*[local-name()='response']/*[local-name()='status']/*[local-name()='ObjectStatus']/ErrorCode")
		if err != nil {
			return nil, err
		}
		logging.LogDebugf("repo/dp/listObjects(), objectNamesAndStatuses: %v", objectNamesAndStatuses)
		parentDir := model.Item{Name: "..", Config: itemConfig.Parent}

		items := make(model.ItemList, len(objectNamesAndStatuses[0]))
		items = append(items, parentDir)
		for idx, objectNameFromStatus := range objectNamesAndStatuses[0] {
			// For "singleton" intrinsic objects in default domain we can't use name
			// from ObjectStatus.
			objectName := objectNameFromStatus
			if len(objectNames) == 1 {
				objectName = objectNames[0]
			}
			modified := ""
			if objectNamesAndStatuses[1][idx] != objectStatusSaved {
				modified = objectNamesAndStatuses[1][idx]
			}
			opState := ""
			if objectNamesAndStatuses[2][idx] != "up" {
				opState = objectNamesAndStatuses[2][idx]
			}
			itemConfig := model.ItemConfig{Type: model.ItemDpObject,
				Name:        objectName,
				DpAppliance: itemConfig.DpAppliance,
				DpDomain:    itemConfig.DpDomain,
				Path:        objectClassName,
				Parent:      itemConfig}
			itemConfig.DpObjectState.ConfigState = objectNamesAndStatuses[1][idx]
			itemConfig.DpObjectState.OpState = objectNamesAndStatuses[2][idx]
			itemConfig.DpObjectState.AdminState = objectNamesAndStatuses[3][idx]
			itemConfig.DpObjectState.EventCode = objectNamesAndStatuses[4][idx]
			itemConfig.DpObjectState.ErrorCode = objectNamesAndStatuses[5][idx]
			item := model.Item{Name: objectName, Modified: modified, Size: opState, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/listObjects(), items: %v", items)
		return items, nil
	default:
		logging.LogDebug("repo/dp/listObjects(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

// listStatusClasses lists all status classes used in current DataPower domain.
func (r *dpRepo) listStatusClasses(currentView *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listStatusClasses(%v)", currentView)

	if currentView.DpAppliance == "" {
		return nil, errs.Error("Can't get status class list if DataPower appliance is not selected.")
	}

	if currentView.DpDomain == "" {
		return nil, errs.Error("Can't get status class list if DataPower domain is not selected.")
	}

	classNames := make([]string, 0)
	classSizeMap := make(map[string]int)
	// Can be find out only for SOMA interface.
	classModifiedMap := make(map[string]bool)
	var err error

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		responseJSON, err := r.restGet("/mgmt/status/")
		if err != nil {
			return nil, err
		}
		doc, err := jsonquery.Parse(strings.NewReader(responseJSON))
		if err != nil {
			logging.LogDebug("Error parsing JSON.", err)
			return nil, err
		}
		query := "/_links/*"
		resultNodes := jsonquery.Find(doc, query)
		if resultNodes == nil {
			logging.LogDebugf("Can't find '%s' in JSON:\n'%s'", query, responseJSON)
			return nil, errs.Errorf("Unexpected JSON, can't find '%s'.", query)
		}
		for _, node := range resultNodes {
			className := node.Data
			if className != "self" {
				classNames = append(classNames, className)
			}
		}
	case config.DpInterfaceSoma:
		somaRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
	  <man:request domain="%s">
	    <man:get-status/>
	  </man:request>
	</soapenv:Body>
</soapenv:Envelope>`, currentView.DpDomain)
		var somaResponse string
		somaResponse, err = r.soma(somaRequest)
		if err != nil {
			return nil, err
		}

		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}
		query := "//*[local-name()='response']/*[local-name()='status']/*"
		resultNodes := xmlquery.Find(doc, query)
		if resultNodes == nil {
			logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
			return nil, errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
		}

		for idx, node := range resultNodes {
			className := node.Data
			if className == "self" {
				continue
			}
			status := parseStatusFromXML(className, idx, node)

			if _, existingName := classSizeMap[className]; !existingName {
				classNames = append(classNames, className)
			}
			classSizeMap[className]++

			if status.modified != "" {
				classModifiedMap[className] = true
			}
		}

	default:
		r.DpViewMode = model.DpFilestoreMode
		logging.LogDebug("repo/dp/listStatusClasses(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}

	if err != nil {
		return nil, err
	}

	logging.LogDebugf("repo/dp/listStatusClasses(), classNames: %v", classNames)

	items := make(model.ItemList, len(classNames))
	for idx, className := range classNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpStatusClass,
			Name:        className,
			DpAppliance: currentView.DpAppliance,
			DpDomain:    currentView.DpDomain,
			Path:        className,
			Parent:      currentView}
		statusCnt := ""
		if r.dataPowerAppliance.DpManagmentInterface() == config.DpInterfaceSoma {
			statusCnt = fmt.Sprintf("%d", classSizeMap[className])
		}
		modified := ""
		if classModifiedMap[className] {
			modified = "*"
		}
		item := model.Item{Name: className,
			Size:     statusCnt,
			Modified: modified,
			Config:   &itemConfig}
		items[idx] = item
	}

	sort.Sort(items)

	logging.LogDebugf("repo/dp/listStatusClasses(), items: %v", items)
	return items, nil
}

// statusInfo contains basic info for each status (if available). For xsl cache
// it is: xml mgr name, used cache size, cache used.
type statusInfo struct {
	name     string
	size     string
	modified string
}

// parseStatusFromJSON parses JSON node and creates statusInfo struct.
func parseStatusFromJSON(
	statusClassName string, nodeIdx int, node *jsonquery.Node) statusInfo {
	var statusName string
	var size string
	var modified string
	switch statusClassName {
	case "StylesheetCachingSummary":
		statusName = jsonquery.FindOne(node, "/XMLManager/value").InnerText()
		size = jsonquery.FindOne(node, "/CacheCount").InnerText()
	case "DocumentCachingSummary":
		statusName = jsonquery.FindOne(node, "/XMLManager/value").InnerText()
		size = jsonquery.FindOne(node, "/DocCount").InnerText()
	default:
		statusName = fmt.Sprintf("%d", nodeIdx)
	}

	if size != "" && size != "0" {
		modified = "*"
	}

	return statusInfo{statusName, size, modified}
}

// parseStatusFromXML parses XML node and creates statusInfo struct.
func parseStatusFromXML(
	statusClassName string, nodeIdx int, node *xmlquery.Node) statusInfo {
	var statusName string
	var size string
	var modified string
	switch statusClassName {
	case "StylesheetCachingSummary":
		statusName = xmlquery.FindOne(node, "/XMLManager").InnerText()
		size = xmlquery.FindOne(node, "/CacheCount").InnerText()
	case "DocumentCachingSummary":
		statusName = xmlquery.FindOne(node, "/XMLManager").InnerText()
		size = xmlquery.FindOne(node, "/DocCount").InnerText()
	case "DocumentCachingSummaryGlobal":
		statusName = xmlquery.FindOne(node, "/Domain").InnerText() + ": " +
			xmlquery.FindOne(node, "/XMLManager").InnerText()
		size = xmlquery.FindOne(node, "/DocCount").InnerText()
	default:
		statusName = fmt.Sprintf("%d", nodeIdx)
	}

	if size != "" && size != "0" {
		modified = "*"
	}

	return statusInfo{statusName, size, modified}
}

// listStatuses lists all statuses of selected class in current DataPower domain.
func (r *dpRepo) listStatuses(itemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listStatuses(%v)", itemConfig)

	switch itemConfig.Type {
	case model.ItemDpStatusClass:
	default:
		return nil, errs.Error("Can't get status list if no status class is known.")
	}

	statusClassName := itemConfig.Path

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		getStatusesURL := fmt.Sprintf("/mgmt/status/%s/%s",
			itemConfig.DpDomain, itemConfig.Path)
		statusesRespJSON, err := r.restGet(getStatusesURL)
		if err != nil {
			if respErr, ok := err.(errs.UnexpectedHTTPResponse); ok && respErr.StatusCode == 404 {
				return nil, nil
			}
			return nil, err
		}
		logging.LogDebugf("repo/dp/listStatuses(), statusesRespJSON: '%s'", statusesRespJSON)

		jqQueryStatuses := fmt.Sprintf(".%s", statusClassName)
		jqOpStatuses, err := jq.Parse(jqQueryStatuses)
		if err != nil {
			logging.LogDebugf("repo/dp/listStatuses(), error parsing query '%s', %v",
				jqOpStatuses, err)
			return nil, err
		}
		statusesJSON, err := jqOpStatuses.Apply([]byte(statusesRespJSON))
		if err != nil {
			logging.LogDebugf("repo/dp/listStatuses(), error applying query '%s', %v",
				jqQueryStatuses, err)
			return nil, err
		}
		logging.LogDebugf("repo/dp/listStatuses(), statusesJSON: '%s'", statusesJSON)

		doc, err := jsonquery.Parse(bytes.NewReader(statusesJSON))
		if err != nil {
			logging.LogDebugf("Error parsing statuses JSON , err: %v", err)
			return nil, err
		}

		var query string
		firstChar := string(statusesJSON[0:1])
		switch firstChar {
		case "[":
			query = "/*"
		case "{":
			query = "/"
		default:
			return nil, errs.Errorf("Unexpected result for query '%s' on '%s'.",
				query, statusesJSON)
		}

		nodes := jsonquery.Find(doc, query)
		if len(nodes) == 0 {
			return nil, errs.Errorf("No status for class '%s' found.", statusClassName)
		}

		parentDir := model.Item{Name: "..", Config: itemConfig.Parent}
		items := make(model.ItemList, len(nodes))
		items = append(items, parentDir)

		for idx, node := range nodes {
			statusPath := fmt.Sprintf("%d", idx)
			status := parseStatusFromJSON(statusClassName, idx, node)

			itemConfig := model.ItemConfig{Type: model.ItemDpStatus,
				Name:        status.name,
				DpAppliance: itemConfig.DpAppliance,
				DpDomain:    itemConfig.DpDomain,
				Path:        statusPath,
				Parent:      itemConfig}

			item := model.Item{Name: status.name, Size: status.size,
				Modified: status.modified, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/listObjects(), items: %v", items)
		return items, nil
	case config.DpInterfaceSoma:
		somaStatusRequest := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Body>
		<man:request domain="%s">
			<man:get-status class="%s"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`, itemConfig.DpDomain, itemConfig.Path)
		doc, err := r.somaGetDoc(somaStatusRequest)
		if err != nil {
			return nil, err
		}

		query := fmt.Sprintf("//*[local-name()='response']/*[local-name()='status']/*[local-name()='%s']", statusClassName)
		nodes := xmlquery.Find(doc, query)
		if len(nodes) == 0 {
			return nil, errs.Errorf("No status for class '%s' found.", statusClassName)
		}

		parentDir := model.Item{Name: "..", Config: itemConfig.Parent}

		items := make(model.ItemList, len(nodes))
		items = append(items, parentDir)
		for idx, node := range nodes {
			statusPath := fmt.Sprintf("%d", idx)
			status := parseStatusFromXML(statusClassName, idx, node)

			itemConfig := model.ItemConfig{Type: model.ItemDpStatus,
				Name:        status.name,
				DpAppliance: itemConfig.DpAppliance,
				DpDomain:    itemConfig.DpDomain,
				Path:        statusPath,
				Parent:      itemConfig}
			item := model.Item{Name: status.name, Size: status.size,
				Modified: status.modified, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/listStatuses(), items: %v", items)
		return items, nil
	default:
		logging.LogDebug("repo/dp/listStatuses(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}
}

func (r *dpRepo) refreshSomaFiles(viewConfig *model.ItemConfig) error {
	return r.refreshSomaFilesByPath(viewConfig.DpDomain, viewConfig.Path)
}
func (r *dpRepo) refreshSomaFilesByPath(dpDomain, path string) error {
	if r.dataPowerAppliance.SomaUrl != "" {
		filestoreEndIdx := strings.Index(path, ":")
		if filestoreEndIdx == -1 {
			return nil
		}

		dpFilestoreLocation := path[:filestoreEndIdx] + ":"
		err := r.fetchFilestoreIfNeeded(dpDomain, dpFilestoreLocation, true)
		return err
	}

	logging.LogDebug("repo/dp/refreshSomaFilesByPath() - called for non-SOMA.")
	return errs.Error("Internal error - refreshSomaFilesByPath() called for non-SOMA.")
}

func (r *dpRepo) findItemConfigParentDomain(itemConfig *model.ItemConfig) *model.ItemConfig {
	if itemConfig.Type == model.ItemDpDomain {
		return itemConfig
	}
	if itemConfig.Parent == nil {
		return nil
	}
	return r.findItemConfigParentDomain(itemConfig.Parent)
}

func (r *dpRepo) fetchDpDomains() ([]dpDomainInfo, error) {
	logging.LogDebug("repo/dp/fetchDpDomains()")
	domains := make([]dpDomainInfo, 0)

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		// Fetch config of all domains (so we can show if domain is enabled)
		domainsConfigDoc, err := r.restGetDoc("/mgmt/config/default/Domain")
		if err != nil {
			return nil, err
		}
		domainConfigList := jsonquery.Find(domainsConfigDoc, "/Domain/*")

		// Fetch status of all domains (so we can show if domain is saved)
		domainsStatusDoc, err := r.restGetDoc("/mgmt/status/default/DomainStatus")
		if err != nil {
			return nil, err
		}

		for _, n := range domainConfigList {
			domainName := n.SelectElement("name").InnerText()
			domain := dpDomainInfo{name: domainName}

			// .Domain[].name, .Domain[].mAdminState
			mAdminState := n.SelectElement("mAdminState")
			domain.down = mAdminState != nil && mAdminState.InnerText() != "enabled"

			// .DomainStatus[].Domain, .DomainStatus[].SaveNeeded
			saveNeeded := jsonquery.FindOne(domainsStatusDoc, fmt.Sprintf("/DomainStatus/*[Domain='%s']/SaveNeeded", domain.name))
			domain.saveNeeded = saveNeeded != nil && saveNeeded.InnerText() == "on"
			domains = append(domains, domain)
		}
		logging.LogDebugf("repo/dp/fetchDpDomains(), domains: '%v'", domains)
	case config.DpInterfaceSoma:
		somaConfigRequest := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="default">
			<man:get-config class="Domain"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`
		domainsConfigDoc, err := r.somaGetDoc(somaConfigRequest)
		if err != nil {
			return nil, err
		}
		domainConfigList := xmlquery.Find(domainsConfigDoc,
			"//*[local-name()='response']/*[local-name()='config']/*[local-name()='Domain']")

		somaStatusRequest := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	xmlns:man="http://www.datapower.com/schemas/management">
	<soapenv:Header/>
	<soapenv:Body>
		<man:request domain="default">
			<man:get-status class="DomainStatus"/>
		</man:request>
	</soapenv:Body>
</soapenv:Envelope>`
		domainsStatusDoc, err := r.somaGetDoc(somaStatusRequest)
		if err != nil {
			return nil, err
		}

		for _, n := range domainConfigList {
			domainName := n.SelectAttr("name")
			domain := dpDomainInfo{name: domainName}

			mAdminState := n.SelectElement("mAdminState")
			domain.down = mAdminState != nil && mAdminState.InnerText() != "enabled"

			saveNeeded := xmlquery.FindOne(domainsStatusDoc,
				fmt.Sprintf("//*[local-name()='response']/*[local-name()='status']/*[local-name()='DomainStatus']/Domain[text()='%s']/../SaveNeeded", domain.name))
			domain.saveNeeded = saveNeeded != nil && saveNeeded.InnerText() == "on"

			domains = append(domains, domain)
		}
		logging.LogDebugf("repo/dp/fetchDpDomains(), domains: '%v'", domains)
		if err != nil {
			return nil, err
		}
	default:
		logging.LogDebug("repo/dp/fetchDpDomains(), using neither REST neither SOMA.")
		return nil, errs.Error("DataPower management interface not set.")
	}

	return domains, nil
}

func (r *dpRepo) restPostForResult(urlPath, postBody, checkQuery, checkExpected, resultQuery string) (result, responseJSON string, err error) {
	responseJSON, err = r.rest(urlPath, "POST", postBody)
	if err != nil {
		return "", "", err
	}

	doc, err := jsonquery.Parse(strings.NewReader(responseJSON))
	if err != nil {
		logging.LogDebug("Error parsing response JSON.", err)
		return "", responseJSON, err
	}

	expectedNode := jsonquery.FindOne(doc, checkQuery)
	if expectedNode == nil {
		logging.LogDebugf("Can't find '%s' in response:\n'%s", checkQuery, responseJSON)
		return "", responseJSON, errs.Error("Unexpected response from server.")
	}

	gotResult := expectedNode.InnerText()
	if gotResult != checkExpected {
		logging.LogDebugf("Unexpected result for '%s' ('%s') in response:\n'%s", checkQuery, gotResult, responseJSON)
		return "", responseJSON, errs.Errorf("Unexpected response from server ('%s').", gotResult)
	}

	resultNode := jsonquery.FindOne(doc, resultQuery)
	if resultNode == nil {
		logging.LogDebugf("Can't find '%s' in response:\n'%s", resultQuery, responseJSON)
		return "", responseJSON, errs.Error("Unexpected response from server.")
	}
	result = resultNode.InnerText()

	return result, responseJSON, nil
}

func (r *dpRepo) restGetForOneResult(urlPath, resultQuery string) (result, responseJSON string, err error) {
	responseJSON, err = r.restGet(urlPath)
	if err != nil {
		return "", "", err
	}

	result, err = parseJSONFindOne(responseJSON, resultQuery)

	return result, responseJSON, err
}

// parseJSONFindOne query JSON and returns a string value.
func parseJSONFindOne(json, query string) (string, error) {
	doc, err := jsonquery.Parse(strings.NewReader(json))
	if err != nil {
		logging.LogDebug("Error parsing JSON.", err)
		return "", err
	}

	resultNode := jsonquery.FindOne(doc, query)
	if resultNode == nil {
		logging.LogDebugf("Can't find '%s' in JSON:\n'%s'", query, json)
		return "", errs.Errorf("Unexpected JSON, can't find '%s'.", query)
	}

	return resultNode.InnerText(), nil
}

// restGetForListResult makes REST call and parses JSON response.
func (r *dpRepo) restGetForListResult(urlPath, resultQuery string) (result []string, responseJSON string, err error) {
	responseJSON, err = r.restGet(urlPath)
	if err != nil {
		return nil, "", err
	}

	result, err = parseJSONFindList(responseJSON, resultQuery)

	return result, responseJSON, err
}

// restGetForListsResult makes REST call and parses JSON response multiple times.
func (r *dpRepo) restGetForListsResult(urlPath string, resultQueries ...string) (results [][]string, responseJSON string, err error) {
	responseJSON, err = r.restGet(urlPath)
	if err != nil {
		return nil, "", err
	}

	results = make([][]string, len(resultQueries))
	for queryIdx, resultQuery := range resultQueries {
		result, err := parseJSONFindList(responseJSON, resultQuery)
		if err != nil {
			return nil, "", err
		}

		results[queryIdx] = result
	}

	return results, responseJSON, err
}

// parseJSONFindList query JSON and returns array of strings.
func parseJSONFindList(json, query string) ([]string, error) {
	results, err := parseJSONFindLists(json, query)

	if err != nil {
		return nil, err
	}

	return results[0], nil
}

// parseJSONFindLists performs multiple queries on JSON and returns
// array of array of strings (for each query one result array).
func parseJSONFindLists(json string, queries ...string) ([][]string, error) {
	doc, err := jsonquery.Parse(strings.NewReader(json))
	if err != nil {
		logging.LogDebug("Error parsing JSON.", err)
		return nil, err
	}

	results := make([][]string, len(queries))
	for queryIdx, query := range queries {
		resultNodes := jsonquery.Find(doc, query)
		if resultNodes == nil {
			logging.LogDebugf("Can't find '%s' in JSON:\n'%s'", query, json)
			return nil, errs.Errorf("Unexpected JSON, can't find '%s'.", query)
		}

		result := make([]string, len(resultNodes))
		for nodeIdx, node := range resultNodes {
			result[nodeIdx] = node.InnerText()
		}

		results[queryIdx] = result
	}

	return results, nil
}

// parseSOMAFindOne query soma response and returns strings value.
func parseSOMAFindOne(somaResponse, query string) (string, error) {
	doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
	if err != nil {
		logging.LogDebug("Error parsing response SOAP.", err)
		return "", err
	}
	resultNode := xmlquery.FindOne(doc, query)
	if resultNode == nil {
		logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
		return "", errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
	}

	return strings.TrimSpace(resultNode.InnerText()), nil
}

// parseSOMAFindList query soma response and returns array of strings.
func parseSOMAFindList(somaResponse, query string) ([]string, error) {
	results, err := parseSOMAFindLists(somaResponse, query)

	if err != nil {
		return nil, err
	}

	return results[0], nil
}

// parseSOMAFindLists perform multiple querys on soma response and returns
// array of array of strings (for each query one response).
func parseSOMAFindLists(somaResponse string, querys ...string) ([][]string, error) {
	doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
	if err != nil {
		logging.LogDebug("Error parsing response SOAP.", err)
		return nil, err
	}
	results := make([][]string, len(querys))
	for queryIdx, query := range querys {
		resultNodes := xmlquery.Find(doc, query)
		if resultNodes == nil {
			logging.LogDebugf("Can't find '%s' in SOMA response:\n'%s'", query, somaResponse)
			return nil, errs.Errorf("Unexpected SOMA, can't find '%s'.", query)
		}

		result := make([]string, len(resultNodes))
		for nodeIdx, node := range resultNodes {
			result[nodeIdx] = node.InnerText()
		}

		results[queryIdx] = result
	}

	return results, nil
}

// cleanJSONObject removes JSON parts which cause errors when we try to PUT updated
// JSON definition to DataPower - it removes "_links" part and all "href" values.
func cleanJSONObject(objectJSON string) ([]byte, error) {
	logging.LogTracef("repo/dp/cleanJSONObject('%s')", objectJSON)

	cleanedJSON := removeJSONKey(objectJSON, "_links")
	cleanedJSON = removeJSONKey(cleanedJSON, "href")

	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(cleanedJSON), "", "  ")
	cleanedJSON = prettyJSON.String()

	logging.LogTracef("repo/dp/cleanJSONObject(), cleanedJSON: '%s'", cleanedJSON)
	return []byte(cleanedJSON), nil
}

// removeJSONKey removes key and it's value from inputJSON
func removeJSONKey(inputJSON, keyName string) string {
	// Find JSON key and use it as starting point for removal (if key is found)
	keyQuoted := fmt.Sprintf(`"%s"`, keyName)
	keyStartIdx := strings.Index(inputJSON, keyQuoted)
	if keyStartIdx != -1 {
		// remove preceeding ',' char if found (go back to first non-white character)
		preceedingIdxRemoved := false
		idx := keyStartIdx - 1
		for ; inputJSON[idx] == ' ' || inputJSON[idx] == '\t' || inputJSON[idx] == '\n' || inputJSON[idx] == '\r'; idx-- {
		}
		if inputJSON[idx] == ',' {
			keyStartIdx = idx
			preceedingIdxRemoved = true
		}

		// Start to create result cleanedJSON with everything before key which we are removing
		cleanedJSON := inputJSON[:keyStartIdx]

		// Find first char where value for key is defined one of:
		// string - '"', object - '{' or value - '['
		rest := inputJSON[keyStartIdx:]
		keyColonIdx := strings.Index(rest, ":")
		idx = keyColonIdx + 1
		for ; rest[idx] == ' ' || rest[idx] == '\t' || rest[idx] == '\n' || rest[idx] == '\r'; idx++ {
		}
		firstValueChar := rest[idx]

		// Find end of value definition for string it is next quote '"', for arrays and
		// object we have to count nesting level (here we don't check if arrays and
		// objects are properly nested one within other - we just count begin/end number
		// of array and object definitions).
		idx++
		valueCharLevel := 1
		lastChar := " "[0]
		for ; valueCharLevel > 0 && idx < len(rest); idx++ {
			// fmt.Printf("removeJSONKey(), idx: %d, level: %d rest: '%s'\n", idx, valueCharLevel, rest[idx:])
			if lastChar != '\\' {
				switch firstValueChar {
				case '"':
					if rest[idx] == '"' {
						valueCharLevel = 0
					}
				case '{', '[':
					switch rest[idx] {
					case '{', '[':
						valueCharLevel++
					case '}', ']':
						valueCharLevel--
					}
				}
			}
			lastChar = rest[idx]
		}
		lastValueIdx := idx

		// Remove following ',' char if it is found and preeceeding ',' was not removed
		if !preceedingIdxRemoved {
			idx = lastValueIdx
			for ; rest[idx] == ' ' || rest[idx] == '\t' || rest[idx] == '\n' || rest[idx] == '\r'; idx++ {
			}
			if rest[idx] == ',' {
				lastValueIdx = idx + 1
			}
		}
		cleanedJSON = cleanedJSON + rest[lastValueIdx:]

		// Repeat process on result - more than one key could be found.
		// Potential problem could be stack overflow if too many of keys are found
		// - recursion could be too deep. If that becomes problem this can easily be
		// refactored.
		return removeJSONKey(cleanedJSON, keyName)
	}

	return inputJSON
}

// cleanXML removes XML parts which cause errors when we try to send updated XML
// definition to DataPower - it removes namespace attributes from root node &
// removes "persisted" attribute from all nodes.
func cleanXML(inputXML string) (string, error) {
	logging.LogTracef("repo/dp/cleanXML('%s')", inputXML)

	// Remove namespace declaration from root element
	// <XMLFirewallService xmlns:_xmlns="xmlns" _xmlns:env="http://www.w3.org/2003/05/soap-envelope" name="parse-cert">
	re := regexp.MustCompile(` [^:^ ]+:[^:^ ]+="[^"]+"`)
	outputXML := re.ReplaceAllString(inputXML, "")

	// Remove persisted attribute from all elements
	// <DebugMode persisted="false">off</DebugMode>
	re = regexp.MustCompile(` persisted="[a-z]+"`)
	outputXML = re.ReplaceAllString(outputXML, "")

	// Remove XMLFirewall from: MgmtInterface, WebB2BViewer & WebGUI
	// otherwise update doesn't work. (?s) - match newlines.
	re = regexp.MustCompile(`(?s)(<(MgmtInterface|WebB2BViewer|WebGUI) .+?)(<XMLFirewall .+?</XMLFirewall>)(.*?</(MgmtInterface|WebB2BViewer|WebGUI)>)`)
	// group 1 is all before XMLFirewall, group 2 is management start element
	// group 3 is XMLFirewall, group 4 is all after XMLFirewall.
	outputXML = re.ReplaceAllString(outputXML, "${1}${4}")

	outputXMLBytes, err := mxj.BeautifyXml([]byte(outputXML), "", "  ")
	if err != nil {
		return "", err
	}
	outputXML = string(outputXMLBytes)
	logging.LogTracef("repo/dp/cleanXML(), outputXML: '%s'", outputXML)
	return outputXML, err
}

// splitOnFirst splits given string in two parts (prefix, suffix) where prefix is
// part of the string before first found splitterString and suffix is part of string
// after first found splitterString.
func splitOnFirst(wholeString string, splitterString string) (string, string) {
	prefix := wholeString
	suffix := ""

	lastIdx := strings.Index(wholeString, splitterString)
	if lastIdx != -1 {
		prefix = wholeString[:lastIdx]
		suffix = wholeString[lastIdx+1:]
	}

	return prefix, suffix
}

// splitOnLast splits given string in two parts (prefix, suffix) where prefix is
// part of the string before last found splitterString and suffix is part of string
// after last found splitterString.
func splitOnLast(wholeString string, splitterString string) (string, string) {
	prefix := wholeString
	suffix := ""

	lastIdx := strings.LastIndex(wholeString, splitterString)
	if lastIdx != -1 {
		prefix = wholeString[:lastIdx]
		suffix = wholeString[lastIdx+1:]
	}

	return prefix, suffix
}

// InitNetworkSettings initializes DataPower client network configuration.
func (r *dpRepo) InitNetworkSettings(applianceName string,
	dpa config.DataPowerAppliance) error {
	logging.LogDebugf("repo/dp/InitNetworkSettings(%v)", dpa)
	r.dataPowerAppliance = dpApplicance{name: applianceName, DataPowerAppliance: dpa}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if r.dataPowerAppliance.Proxy != "" {
		proxyURL, err := url.Parse(r.dataPowerAppliance.Proxy)
		if err != nil {
			logging.LogDebug("Couldn't initialize network settings to access DataPower.", err)
			return err
		}
		http.DefaultTransport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	}
	return nil
}

// rest makes http request from relative URL path given, method and body.
func (r *dpRepo) rest(urlPath, method, body string) (string, error) {
	fullURL := r.dataPowerAppliance.RestUrl + urlPath
	return r.httpRequest(fullURL, method, body)
}

// restGetDoc makes DataPower REST GET request and returns parsed JSON doc.
func (r *dpRepo) restGetDoc(urlPath string) (*jsonquery.Node, error) {
	logging.LogDebugf("repo/dp/restGetDoc('%s')", urlPath)
	bodyString, err := r.restGet(urlPath)
	if err != nil {
		return nil, err
	}
	doc, err := jsonquery.Parse(strings.NewReader(bodyString))
	if err != nil {
		logging.LogDebugf("Error parsing response JSON, err: %v", err)
		return nil, err
	}

	return doc, nil
}

// restGet makes DataPower REST GET request.
func (r *dpRepo) restGet(urlPath string) (string, error) {
	return r.rest(urlPath, "GET", "")
}

// amp makes DataPower AMP request.
func (r *dpRepo) amp(body string) (string, error) {
	return r.httpRequest(r.dataPowerAppliance.SomaUrl+"/service/mgmt/amp/1.0", "POST", body)
}

// soma makes DataPower SOMA request.
func (r *dpRepo) soma(body string) (string, error) {
	return r.httpRequest(r.dataPowerAppliance.SomaUrl+"/service/mgmt/current", "POST", body)
}

// somaGetDoc makes DataPower SOMA request and returns parsed XML doc.
func (r *dpRepo) somaGetDoc(body string) (*xmlquery.Node, error) {
	logging.LogDebugf("repo/dp/somaGetDoc('%s')", body)
	bodyString, err := r.soma(body)
	if err != nil {
		return nil, err
	}
	if bodyString == "" {
		logging.LogDebug("No response XML returned for SOMA call.")
		return nil, errs.Error("No response XML returned for SOMA call.")
	}
	doc, err := xmlquery.Parse(strings.NewReader(bodyString))
	if err != nil {
		logging.LogDebugf("Error parsing response XML, err: %v", err)
		return nil, err
	}

	return doc, nil
}

type requester interface {
	httpRequest(dpa dpApplicance, urlFullPath, method, body string) (string, error)
}

type netRequester struct{}

func (nr netRequester) httpRequest(dpa dpApplicance, urlFullPath, method, body string) (string, error) {
	logging.LogTracef("repo/dp/httpRequest(%s, %s, '%s')", urlFullPath, method, body)

	client := &http.Client{}
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlFullPath, bodyReader)
	if err != nil {
		logging.LogDebug("repo/dp/httpRequest() - Can't prepare request: ", err)
		return "", err
	}

	req.SetBasicAuth(dpa.Username, dpa.DpPlaintextPassword())
	resp, err := client.Do(req)

	if err != nil {
		logging.LogDebug("repo/dp/httpRequest() - Can't send request: ", err)
		return "", err
		// 2019/10/22 08:39:14 dp Can't send request: Post https://10.123.56.55:5550/service/mgmt/current: dial tcp 10.123.56.55:5550: i/o timeout
		//exit status 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted ||
		resp.StatusCode == http.StatusCreated {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logging.LogDebug("repo/dp/httpRequest() - Can't read response: ", err)
			return "", err
		}
		logging.LogTracef("repo/dp/httpRequest() - httpResponse: '%s'", string(bodyBytes))
		return string(bodyBytes), nil
	}
	// logging.LogTracef("repo/dp/httpRequest() - resp.StatusCode: '%d'", resp.StatusCode)
	// if resp.StatusCode == 403 || resp.StatusCode == 404 {
	// 	return ""
	// }
	logging.LogDebugf("repo/dp/httpRequest() - HTTP %s call to '%s' returned HTTP StatusCode %v (%s)",
		method, urlFullPath, resp.StatusCode, resp.Status)
	return "", errs.UnexpectedHTTPResponse{StatusCode: resp.StatusCode, Status: resp.Status}
}

// httpRequest makes DataPower HTTP request.
func (r *dpRepo) httpRequest(urlFullPath, method, body string) (string, error) {
	return r.req.httpRequest(r.dataPowerAppliance, urlFullPath, method, body)
}

// makeRestPath creates DataPower REST path to given domain.
func makeRestPath(dpDomain, filePath string) string {
	logging.LogDebugf("repo/dp/makeRestPath('%s', '%s')", dpDomain, filePath)
	currRestFilePath := strings.Replace(filePath, ":", "", 1)
	return "/mgmt/filestore/" + dpDomain + "/" + currRestFilePath
}
