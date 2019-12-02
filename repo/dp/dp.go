package dp

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// dpRepo contains basic DataPower repo information and implements Repo interface.
type dpRepo struct {
	name               string
	dpFilestoreXmls    map[string]string
	invalidateCache    bool
	dataPowerAppliance config.DataPowerAppliance
	ObjectConfigMode   bool
}

// Repo is instance or DataPower repo/Repo interface implementation used for all
// operations on DataPower except syncing local filesystem to DataPower.
var Repo = dpRepo{name: "DataPower", dpFilestoreXmls: make(map[string]string)}

// SyncRepo is instance or DataPower repo/Repo interface implementation used for
// syncing local directory to DataPower directory.
var SyncRepo = dpRepo{name: "SyncDataPower", dpFilestoreXmls: make(map[string]string)}

func (r *dpRepo) String() string {
	return r.name
}

func (r *dpRepo) GetInitialItem() (model.Item, error) {
	logging.LogDebugf("repo/dp/GetInitialItem(), dataPowerAppliance: %#v", r.dataPowerAppliance)
	var initialConfig model.ItemConfig
	initialConfigTop := model.ItemConfig{Type: model.ItemNone}
	if r.dataPowerAppliance.RestUrl != "" || r.dataPowerAppliance.SomaUrl != "" || r.dataPowerAppliance.Username != "" {
		initialConfig = model.ItemConfig{
			Type:        model.ItemDpConfiguration,
			DpAppliance: config.CurrentApplianceName,
			DpDomain:    r.dataPowerAppliance.Domain,
			Parent:      &initialConfigTop}
	} else {
		initialConfig = initialConfigTop
	}
	logging.LogDebugf("repo/dp/GetInitialItem() initialConfig: %#v", initialConfig)
	initialItem := model.Item{Config: &initialConfig}

	return initialItem, nil
}

func (r *dpRepo) GetTitle(itemToShow *model.ItemConfig) string {
	logging.LogDebugf("repo/dp/GetTitle(%v)", itemToShow)
	dpDomain := itemToShow.DpDomain
	currPath := itemToShow.Path

	var url string
	if r.dataPowerAppliance.RestUrl != "" {
		url = r.dataPowerAppliance.RestUrl
	} else if r.dataPowerAppliance.SomaUrl != "" {
		url = r.dataPowerAppliance.SomaUrl
	} else {
		logging.LogDebug("repo/dp/GetTitle(), using neither REST neither SOMA.")
	}

	return fmt.Sprintf("%s @ %s (%s) %s", r.dataPowerAppliance.Username, url, dpDomain, currPath)
}
func (r *dpRepo) GetList(itemToShow *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/GetList(%v)", itemToShow)

	if r.ObjectConfigMode {
		switch itemToShow.Type {
		case model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory:
			return r.listObjectClasses(itemToShow)
		case model.ItemDpObjectClass:
			return r.listObjects(itemToShow)
		default:
			logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item for ObjectConfigMode: %t.",
				itemToShow, r.ObjectConfigMode)
			r.ObjectConfigMode = false
			return nil, errs.Errorf("Can't show object view if DataPower domain is not selected.")
		}
	} else {
		switch itemToShow.Type {
		case model.ItemNone:
			r.dataPowerAppliance = config.DataPowerAppliance{}
			return listAppliances()
		case model.ItemDpConfiguration:
			r.dataPowerAppliance = config.Conf.DataPowerAppliances[itemToShow.DpAppliance]
			if r.dataPowerAppliance.Password == "" {
				r.dataPowerAppliance.SetDpPlaintextPassword(config.DpTransientPasswordMap[itemToShow.DpAppliance])
			}
			if itemToShow.DpDomain != "" {
				return r.listFilestores(itemToShow)
			}
			return r.listDomains(itemToShow)
		case model.ItemDpDomain:
			return r.listFilestores(itemToShow)
		case model.ItemDpFilestore:
			return r.listDpDir(itemToShow)
		case model.ItemDirectory:
			return r.listDpDir(itemToShow)
		default:
			logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item for ObjectConfigMode: %t.",
				itemToShow, r.ObjectConfigMode)
			return model.ItemList{}, nil
		}
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

	return r.GetFileByPath(currentView.DpDomain, filePath)
}
func (r *dpRepo) GetFileByPath(dpDomain, filePath string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetFile('%s', '%s')", dpDomain, filePath)

	if r.dataPowerAppliance.RestUrl != "" {
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
	} else if r.dataPowerAppliance.SomaUrl != "" {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + dpDomain + "\">" +
			"<dp:get-file name=\"" + filePath + "\"/></dp:request></soapenv:Body></soapenv:Envelope>"
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
	}

	logging.LogDebug("repo/dp/GetFile(), using neither REST neither SOMA.")
	return nil, errs.Error("DataPower management interface not set.")
}

func (r *dpRepo) UpdateFile(currentView *model.ItemConfig, fileName string, newFileContent []byte) (bool, error) {
	logging.LogDebugf("repo/dp/UpdateFile(%s, '%s', ...)\n", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetDpPath(parentPath, fileName)
	return r.UpdateFileByPath(currentView.DpDomain, filePath, newFileContent)
}
func (r *dpRepo) UpdateFileByPath(dpDomain, filePath string, newFileContent []byte) (bool, error) {
	logging.LogDebugf("repo/dp/UpdateFileByPath('%s', '%s', ...)", dpDomain, filePath)
	fileType, err := r.GetFileTypeByPath(dpDomain, filePath, ".")
	logging.LogDebugf("repo/dp/UpdateFileByPath() fileType: %s", fileType)
	if err != nil {
		return false, err
	}

	if r.dataPowerAppliance.RestUrl != "" {
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
	} else if r.dataPowerAppliance.SomaUrl != "" {
		switch fileType {
		case model.ItemNone, model.ItemFile:
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + dpDomain + "\">" +
				"<dp:set-file name=\"" + filePath + "\">" + base64.StdEncoding.EncodeToString(newFileContent) + "</dp:set-file>" +
				"</dp:request></soapenv:Body></soapenv:Envelope>"
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
	}

	logging.LogDebug("repo/dp/UpdateFileByPath(), using neither REST neither SOMA.")
	return false, errs.Error("DataPower management interface not set.")
}

func (r *dpRepo) GetFileType(viewConfig *model.ItemConfig, parentPath, fileName string) (model.ItemType, error) {
	logging.LogDebug(fmt.Sprintf("repo/dp/getFileType(%v, '%s', '%s')\n", viewConfig, parentPath, fileName))
	dpDomain := viewConfig.DpDomain

	return r.GetFileTypeByPath(dpDomain, parentPath, fileName)
}

func (r *dpRepo) GetFileTypeByPath(dpDomain, parentPath, fileName string) (model.ItemType, error) {
	logging.LogDebug(fmt.Sprintf("repo/dp/GetFileTypeByPath('%s', '%s', '%s')\n", dpDomain, parentPath, fileName))
	filePath := paths.GetDpPath(parentPath, fileName)

	if r.dataPowerAppliance.RestUrl != "" {
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
	} else if r.dataPowerAppliance.SomaUrl != "" {
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
		case dpDomain == "":
			return model.ItemDpDomain, nil
		}
	}

	logging.LogDebug("repo/dp/GetFileTypeByPath(), using neither REST neither SOMA.")
	return model.ItemNone, errs.Error("DataPower management interface not set.")
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
		if r.dataPowerAppliance.RestUrl != "" {
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
		} else if r.dataPowerAppliance.SomaUrl != "" {
			dirPath := r.GetFilePath(parentPath, dirName)
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + dpDomain + "\">" +
				"<dp:do-action><CreateDir><Dir>" + dirPath + "</Dir></CreateDir></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
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
			logging.LogDebug("repo/dp/CreateDirByPath() - %s", errMsg)
			return false, errs.Error(errMsg)
		}
		logging.LogDebug("repo/dp/CreateDirByPath(), using neither REST neither SOMA.")
		return false, errs.Error("DataPower management interface not set.")
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
	filePath := r.GetFilePath(parentPath, fileName)

	switch itemType {
	case model.ItemDpConfiguration:
		// deleting DataPower configuration
		config.Conf.DeleteDpApplianceConfig(fileName)
		return true, nil
	case model.ItemDirectory, model.ItemFile:
		switch {
		case r.dataPowerAppliance.RestUrl != "":
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
		case r.dataPowerAppliance.SomaUrl != "":
			fileType, err := r.GetFileType(currentView, parentPath, fileName)
			if err != nil {
				return false, err
			}
			var somaRequest string
			switch fileType {
			case model.ItemDirectory:
				somaRequest = "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
					"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + currentView.DpDomain + "\">" +
					"<dp:do-action><RemoveDir><Dir>" + filePath + "</Dir></RemoveDir></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
			case model.ItemFile:
				somaRequest = "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
					"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + currentView.DpDomain + "\">" +
					"<dp:do-action><DeleteFile><File>" + filePath + "</File></DeleteFile></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
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
			Path:        paths.GetDpPath(parentView.Path, dirFsName),
			DpAppliance: dpView.DpAppliance,
			DpDomain:    dpView.DpDomain,
			DpFilestore: dpFilestore,
			Parent:      parentView}
		parentView = resultView
	}

	return resultView, nil
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
				fileB64, err := parseJsonFindOne(exportResponseJSON, "/result/file")
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
		doc, err := xmlquery.Parse(strings.NewReader(backupResponseSoma))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}
		backupFileNode := xmlquery.FindOne(doc, "//*[local-name()='file']")
		backupFileB64 := backupFileNode.InnerText()

		backupBytes, err := base64.StdEncoding.DecodeString(backupFileB64)
		if err != nil {
			logging.LogDebug("repo/dp/ExportDomain() - Error decoding base64 file.", err)
			return nil, err
		}

		// 2. Extract just given domain backup archive
		backupBytesReader := bytes.NewReader(backupBytes)
		backupZipReader, err := zip.NewReader(backupBytesReader, int64(len(backupBytes)))
		if err != nil {
			logging.LogDebug("repo/dp/ExportDomain() - Error unzipping backup archive.", err)
			return nil, err
		}
		for idx, file := range backupZipReader.File {
			logging.LogDebugf("repo/dp/ExportDomain() - file[%d] : '%s'.", idx, file.Name)
			if file.Name == domainName+".zip" {
				domainBackupBytes := make([]byte, file.UncompressedSize64)
				domainReader, err := file.Open()
				if err != nil {
					logging.LogDebug("repo/dp/ExportDomain() - Error opening domain from backup archive for reading.", err)
					return nil, err
				}
				defer domainReader.Close()

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

// GetObject fetches DataPower object configuration.
func (r *dpRepo) GetObject(itemConfig *model.ItemConfig, objectName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetObject(%v, '%s')", itemConfig, objectName)
	// https://localhost:5554/mgmt/status/tmp/ObjectStatus
	// curl -k -u admin:admin https://localhost:5554/mgmt/ | jq .
	// curl -k -u admin:admin https://localhost:5554/mgmt/config/ | jq . | less
	// curl -k -u admin:admin https://localhost:5554/mgmt/config/tmp/XMLFirewallService | jq . | less

	switch itemConfig.Type {
	case model.ItemDpObject:
	default:
		return nil, errs.Errorf("Can't get object when item is of type %s.",
			itemConfig.Type.UserFriendlyString())
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		getObjectURL := fmt.Sprintf("/mgmt/config/%s/%s/%s",
			itemConfig.DpDomain, itemConfig.Path, objectName)
		objectJSON, err := r.restGet(getObjectURL)
		if err != nil {
			return nil, err
		}

		logging.LogDebugf("repo/dp/GetObject(), objectJSON: '%s'", objectJSON)
		var prettyJSON bytes.Buffer
		json.Indent(&prettyJSON, []byte(objectJSON), "", "  ")
		return prettyJSON.Bytes(), nil
	// case config.DpInterfaceSoma:
	default:
		return nil, errs.Error("Object mode is supported only for REST management interface.")
	}
}

// SetObject updates DataPower object configuration.
func (r *dpRepo) SetObject(itemConfig *model.ItemConfig, objectName string, objectContent []byte) error {
	logging.LogDebugf("repo/dp/SetObject(%v, '%s', ...)", itemConfig, objectName)

	switch itemConfig.Type {
	case model.ItemDpObject:
	default:
		return errs.Errorf("Can't set object when item is of type %s.",
			itemConfig.Type.UserFriendlyString())
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		getObjectURL := fmt.Sprintf("/mgmt/config/%s/%s/%s",
			itemConfig.DpDomain, itemConfig.Path, objectName)
		resultJson, err := r.rest(getObjectURL, "PUT", string(objectContent))
		if err != nil {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), resultJson: '%s'", resultJson)
		errorMessage, err := parseJsonFindOne(resultJson, "/error")
		if err != nil && err.Error() != "Unexpected JSON, can't find '/error'." {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), errorMessage: '%s'", errorMessage)
		successMessage, err := parseJsonFindOne(resultJson, fmt.Sprintf("/%s", objectName))
		if err != nil {
			return err
		}
		logging.LogDebugf("repo/dp/SetObject(), successMessage: '%s'", successMessage)
		// curl -k -u admin:admin https://localhost:5554/mgmt/config/tmp/XMLFirewallService/get_internal_js_xmlfw/LocalPort -d '{"LocalPort":"10009"}' -X PUT | jq .
		// curl -k -u admin:admin https://localhost:5554/mgmt/config/tmp/XMLFirewallService/get_internal_js_xmlfw -d '{"XMLFirewallService":{"name": "get_internal_js_xmlfw","LocalPort":"10001"}}' -X PUT | jq .
		return err
	// case config.DpInterfaceSoma:
	default:
		return errs.Error("Object mode is supported only for REST management interface.")
	}
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
			DpAppliance: name,
			DpDomain:    config.Domain,
			Parent:      &appliancesConfig}
		items[idx] = model.Item{Name: name, Config: &itemConfig}
		idx = idx + 1
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
	domainNames, err := r.fetchDpDomains()
	if err != nil {
		return nil, err
	}
	logging.LogDebugf("repo/dp/listDomains('%s'), domainNames: %v", selectedItemConfig, domainNames)

	items := make(model.ItemList, len(domainNames)+1)
	items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

	for idx, name := range domainNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpDomain,
			DpAppliance: selectedItemConfig.DpAppliance,
			DpDomain:    name,
			Parent:      selectedItemConfig}
		items[idx+1] = model.Item{Name: name, Config: &itemConfig}
	}

	sort.Sort(items)

	return items, nil
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *dpRepo) listFilestores(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listFilestores('%s')", selectedItemConfig)
	if r.dataPowerAppliance.RestUrl != "" {
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
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				Path:        filestoreName,
				Parent:      selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items, nil
	} else if r.dataPowerAppliance.SomaUrl != "" {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItemConfig.DpDomain + "\">" +
			"<dp:get-filestore layout-only=\"true\" no-subdirectories=\"true\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		dpFilestoresXML, err := r.soma(somaRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(dpFilestoresXML))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}
		filestoreNameNodes := xmlquery.Find(doc, "//*[local-name()='location']/@name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore,
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				Path:        filestoreName,
				Parent:      selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items, nil
	}

	logging.LogDebug("repo/dp/listFilestores(), using neither REST neither SOMA.")
	return nil, errs.Error("DataPower management interface not set.")
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
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + dpDomain + "\">" +
				"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\" location=\"" + dpFilestoreLocation + "\"/></dp:request>" +
				"</soapenv:Body></soapenv:Envelope>"
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

	if r.dataPowerAppliance.RestUrl != "" {
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
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
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
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				Path:        paths.GetDpPath(selectedItemConfig.Path, fileName),
				Parent:      selectedItemConfig}
			item := model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
			items = append(items, item)
		}

		sort.Sort(items)
		return items, nil
	} else if r.dataPowerAppliance.SomaUrl != "" {
		dpFilestoreLocation, _ := splitOnFirst(selectedItemConfig.Path, "/")
		dpFilestoreIsRoot := !strings.Contains(selectedItemConfig.Path, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node

		// If we open filestore or open file but want to reload - refresh current filestore XML cache.
		err := r.fetchFilestoreIfNeeded(selectedItemConfig.DpDomain, dpFilestoreLocation, dpFilestoreIsRoot)
		if err != nil {
			logging.LogDebug("Error parsing response JSON.", err)
			return nil, err
		}

		if dpFilestoreIsRoot {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXmls[dpFilestoreLocation]))
			if err != nil {
				logging.LogDebug("Error parsing response JSON.", err)
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
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
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
				DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain:    selectedItemConfig.DpDomain,
				Path:        selectedItemConfig.Path,
				Parent:      selectedItemConfig}
			// selectedItemConfig.Path
			items[idx+dirNum] = model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
		}

		sort.Sort(items)
		return items, nil
	} else {
		logging.LogDebug("repo/dp/listFiles(), using neither REST neither SOMA.")
		return model.ItemList{}, errs.Error("DataPower management interface not set.")
	}
}

// listObjectClasses lists all object classes used in current DataPower domain.
func (r *dpRepo) listObjectClasses(currentView *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/GetObjectClassList(%v)", currentView)

	if currentView.DpAppliance == "" {
		return nil, errs.Error("Can't get object class list if DataPower appliance is not selected.")
	}

	if currentView.DpDomain == "" {
		return nil, errs.Error("Can't get object class list if DataPower domain is not selected.")
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		listObjectStatusesURL := fmt.Sprintf("/mgmt/status/%s/ObjectStatus", currentView.DpDomain)
		classNamesWithDuplicates, _, err := r.restGetForListResult(listObjectStatusesURL, "/ObjectStatus//Class")
		if err != nil {
			return nil, err
		}
		classNameMap := make(map[string]bool)
		classNames := make([]string, 0)
		for _, className := range classNamesWithDuplicates {
			if _, oldName := classNameMap[className]; !oldName {
				classNameMap[className] = true
				classNames = append(classNames, className)
			}
		}

		logging.LogDebugf("repo/dp/GetObjectClassList(), classNames: %v", classNames)

		items := make(model.ItemList, len(classNames))
		for idx, className := range classNames {
			itemConfig := model.ItemConfig{Type: model.ItemDpObjectClass,
				DpAppliance: currentView.DpAppliance,
				DpDomain:    currentView.DpDomain,
				Path:        className,
				Parent:      currentView}
			item := model.Item{Name: className, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/GetObjectClassList(), items: %v", items)
		return items, nil
	// case config.DpInterfaceSoma:
	default:
		return nil, errs.Error("Object mode is supported only for REST management interface.")
	}
}

// listObjects lists all objects of selected class in current DataPower domain.
func (r *dpRepo) listObjects(itemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listObjects(%v)", itemConfig)

	switch itemConfig.Type {
	case model.ItemDpObjectClass:
	default:
		return nil, errs.Error("Can't get object list if no object class is known.")
	}

	switch r.dataPowerAppliance.DpManagmentInterface() {
	case config.DpInterfaceRest:
		listObjectsURL := fmt.Sprintf("/mgmt/config/%s/%s", itemConfig.DpDomain, itemConfig.Path)
		objectNameQuery := fmt.Sprintf("/%s//name", itemConfig.Path)
		objectNames, _, err := r.restGetForListResult(listObjectsURL, objectNameQuery)
		if err != nil {
			return nil, err
		}

		logging.LogDebugf("repo/dp/listObjects(), objectNames: %v", objectNames)
		parentDir := model.Item{Name: "..", Config: itemConfig.Parent}

		items := make(model.ItemList, len(objectNames))
		items = append(items, parentDir)
		for idx, objectName := range objectNames {
			itemConfig := model.ItemConfig{Type: model.ItemDpObject,
				DpAppliance: itemConfig.DpAppliance,
				DpDomain:    itemConfig.DpDomain,
				Path:        itemConfig.Path,
				Parent:      itemConfig}
			item := model.Item{Name: objectName, Config: &itemConfig}
			items[idx] = item
		}

		sort.Sort(items)

		logging.LogDebugf("repo/dp/listObjects(), items: %v", items)
		return items, nil
	// case config.DpInterfaceSoma:
	default:
		return nil, errs.Error("Object mode is supported only for REST management interface.")
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

func (r *dpRepo) fetchDpDomains() ([]string, error) {
	logging.LogDebug("repo/dp/fetchDpDomains()")
	domains := make([]string, 0)

	if r.dataPowerAppliance.RestUrl != "" {
		bodyString, err := r.restGet("/mgmt/domains/config/")
		if err != nil {
			return nil, err
		}

		// .domain[].name
		doc, err := jsonquery.Parse(strings.NewReader(bodyString))
		if err != nil {
			logging.LogDebug("Error parsing response JSON.", err)
			return nil, err
		}
		list := jsonquery.Find(doc, "/domain//name")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else if r.dataPowerAppliance.SomaUrl != "" {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
			"<soapenv:Body><dp:GetDomainListRequest xmlns:dp=\"http://www.datapower.com/schemas/appliance/management/1.0\"/></soapenv:Body>" +
			"</soapenv:Envelope>"
		somaResponse, err := r.amp(somaRequest)
		if err != nil {
			return nil, err
		}
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogDebug("Error parsing response SOAP.", err)
			return nil, err
		}
		list := xmlquery.Find(doc, "//*[local-name()='GetDomainListResponse']/*[local-name()='Domain']/text()")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else {
		logging.LogDebug("repo/dp/fetchDpDomains(), using neither REST neither SOMA.")
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

	result, err = parseJsonFindOne(responseJSON, resultQuery)

	return result, responseJSON, err
}

func parseJsonFindOne(json, query string) (string, error) {
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

func (r *dpRepo) restGetForListResult(urlPath, resultQuery string) (result []string, responseJSON string, err error) {
	responseJSON, err = r.restGet(urlPath)
	if err != nil {
		return nil, "", err
	}

	result, err = parseJsonFindList(responseJSON, resultQuery)

	return result, responseJSON, err
}

func parseJsonFindList(json, query string) ([]string, error) {
	doc, err := jsonquery.Parse(strings.NewReader(json))
	if err != nil {
		logging.LogDebug("Error parsing JSON.", err)
		return nil, err
	}

	resultNodes := jsonquery.Find(doc, query)
	if resultNodes == nil {
		logging.LogDebugf("Can't find '%s' in JSON:\n'%s'", query, json)
		return nil, errs.Errorf("Unexpected JSON, can't find '%s'.", query)
	}

	result := make([]string, len(resultNodes))
	for idx, node := range resultNodes {
		result[idx] = node.InnerText()
	}

	return result, nil
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
func (r *dpRepo) InitNetworkSettings(dpa config.DataPowerAppliance) error {
	logging.LogDebug("repo/dp/InitNetworkSettings(%v)", dpa)
	r.dataPowerAppliance = dpa
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

// httpRequest makes DataPower HTTP request.
func (r *dpRepo) httpRequest(urlFullPath, method, body string) (string, error) {
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

	req.SetBasicAuth(r.dataPowerAppliance.Username, r.dataPowerAppliance.DpPlaintextPassword())
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

// makeRestPath creates DataPower REST path to given domain.
func makeRestPath(dpDomain, filePath string) string {
	logging.LogDebugf("repo/dp/makeRestPath('%s', '%s')", dpDomain, filePath)
	currRestFilePath := strings.Replace(filePath, ":", "", 1)
	return "/mgmt/filestore/" + dpDomain + "/" + currRestFilePath
}
