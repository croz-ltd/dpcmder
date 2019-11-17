package dp

import (
	"encoding/base64"
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp/internal/dpnet"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"sort"
	"strings"
)

// dpRepo contains basic DataPower repo information and implements Repo interface.
type dpRepo struct {
	name            string
	dpFilestoreXml  string
	invalidateCache bool
}

// Repo is instance or DataPower repo/Repo interface implementation.
var Repo = dpRepo{name: "DpRepo"}

// InitNetworkSettings initializes DataPower client network configuration.
func InitNetworkSettings() {
	dpnet.InitNetworkSettings()
}

func (r *dpRepo) String() string {
	return r.name
}

func (r *dpRepo) GetInitialItem() model.Item {
	logging.LogDebug("repo/dp/GetInitialItem()")
	var initialConfig model.ItemConfig
	initialConfigTop := model.ItemConfig{Type: model.ItemNone}
	if config.DpUseRest() || config.DpUseSoma() || *config.DpUsername != "" {
		initialConfig = model.ItemConfig{
			Type:        model.ItemDpConfiguration,
			DpAppliance: config.PreviousAppliance,
			DpDomain:    *config.DpDomain,
			Parent:      &initialConfigTop}
	} else {
		initialConfig = initialConfigTop
	}
	initialItem := model.Item{Config: &initialConfig}

	return initialItem
}

func (r *dpRepo) GetTitle(itemToShow model.Item) string {
	logging.LogDebugf("repo/dp/GetTitle(%v)", itemToShow)
	dpDomain := itemToShow.Config.DpDomain
	currPath := itemToShow.Config.Path

	var url string
	if config.DpUseRest() {
		url = *config.DpRestURL
	} else if config.DpUseSoma() {
		url = *config.DpSomaURL
	} else {
		logging.LogDebug("repo/dp/GetTitle(), using neither REST neither SOMA.")
	}

	return fmt.Sprintf("%s @ %s (%s) %s", *config.DpUsername, url, dpDomain, currPath)
}
func (r *dpRepo) GetList(itemToShow *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/GetList(%v)", itemToShow)

	switch itemToShow.Type {
	case model.ItemNone:
		config.ClearDpConfig()
		return listAppliances(), nil
	case model.ItemDpConfiguration:
		config.LoadDpConfig(itemToShow.DpAppliance)
		if itemToShow.DpDomain != "" {
			return r.listFilestores(itemToShow)
		}
		return listDomains(itemToShow)
	case model.ItemDpDomain:
		return r.listFilestores(itemToShow)
	case model.ItemDpFilestore:
		return r.listDpDir(itemToShow)
	case model.ItemDirectory:
		return r.listDpDir(itemToShow)
	default:
		logging.LogDebugf("repo/dp/GetList(%v) - can't get children or item.", itemToShow)
		return model.ItemList{}, nil
	}
}

func (r *dpRepo) InvalidateCache() {
	logging.LogDebugf("repo/dp/InvalidateCache()")
	if config.DpUseSoma() {
		r.invalidateCache = true
	}
}

func (r *dpRepo) GetFile(currentView *model.ItemConfig, fileName string) ([]byte, error) {
	logging.LogDebugf("repo/dp/GetFile(%v, '%s')", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetDpPath(parentPath, fileName)

	if config.DpUseRest() {
		restPath := dpnet.MakeRestPath(currentView.DpDomain, filePath)
		jsonString, err := dpnet.RestGet(restPath)
		if err != nil {
			return nil, err
		}
		// println("jsonString: " + jsonString)

		if jsonString == "" {
			return nil, errs.Error(fmt.Sprintf("Can't fetch file '%s' from '%s'", fileName, parentPath))
		} else {
			doc, err := jsonquery.Parse(strings.NewReader(jsonString))
			if err != nil {
				logging.LogDebug("repo/dp/GetFile() - Error parsing response JSON.", err)
				return nil, err
			}

			// .filestore.location.directory /name
			// work-around - for one directory we get JSON object, for multiple directories we get JSON array
			fileNode := jsonquery.FindOne(doc, "/file")
			if fileNode == nil {
				errMsg := fmt.Sprintf("Can't find file '%s' on path '%s' from JSON response.", fileName, parentPath)
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
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + currentView.DpDomain + "\">" +
			"<dp:get-file name=\"" + filePath + "\"/></dp:request></soapenv:Body></soapenv:Envelope>"
		somaResponse, err := dpnet.Soma(somaRequest)
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
			errMsg := fmt.Sprintf("Can't find file '%s' on path '%s' from SOMA response.", fileName, parentPath)
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

// listAppliances returns ItemList of DataPower appliance Items from configuration.
func listAppliances() model.ItemList {
	appliances := config.Conf.DataPowerAppliances
	logging.LogDebugf("repo/dp/listAppliances(), appliances: %v", appliances)

	appliancesConfig := model.ItemConfig{Type: model.ItemNone}
	items := make(model.ItemList, len(appliances))
	idx := 0
	for name, config := range appliances {
		itemConfig := model.ItemConfig{Type: model.ItemDpConfiguration, DpAppliance: name, DpDomain: config.Domain, Parent: &appliancesConfig}
		items[idx] = model.Item{Name: name, Config: &itemConfig}
		idx = idx + 1
	}

	sort.Sort(items)
	logging.LogDebugf("repo/dp/listAppliances(), items: %v", items)

	return items
}

// listDomains loads DataPower domains from current DataPower.
func listDomains(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listDomains('%s')", selectedItemConfig)
	domainNames, err := fetchDpDomains()
	if err != nil {
		return nil, err
	}
	logging.LogDebugf("repo/dp/listDomains('%s'), domainNames: %v", selectedItemConfig, domainNames)

	items := make(model.ItemList, len(domainNames)+1)
	items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

	for idx, name := range domainNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpDomain,
			DpAppliance: selectedItemConfig.DpAppliance, DpDomain: name, Parent: selectedItemConfig}
		items[idx+1] = model.Item{Name: name, Config: &itemConfig}
	}

	sort.Sort(items)

	return items, nil
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *dpRepo) listFilestores(selectedItemConfig *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/dp/listFilestores('%s')", selectedItemConfig)
	if config.DpUseRest() {
		jsonString, err := dpnet.RestGet("/mgmt/filestore/" + selectedItemConfig.DpDomain)
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
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain: selectedItemConfig.DpDomain, Path: filestoreName, Parent: selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items, nil
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItemConfig.DpDomain + "\">" +
			"<dp:get-filestore layout-only=\"true\" no-subdirectories=\"true\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		dpFilestoresXML, err := dpnet.Soma(somaRequest)
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
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain: selectedItemConfig.DpDomain, Path: filestoreName, Parent: selectedItemConfig}
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

func (r *dpRepo) listFiles(selectedItemConfig *model.ItemConfig) ([]model.Item, error) {
	logging.LogDebugf("repo/dp/listFiles('%s')", selectedItemConfig)

	if config.DpUseRest() {
		items := make(model.ItemList, 0)
		currRestDirPath := strings.Replace(selectedItemConfig.Path, ":", "", 1)
		jsonString, err := dpnet.RestGet("/mgmt/filestore/" + selectedItemConfig.DpDomain + "/" + currRestDirPath)
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
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: dirDpPath, Parent: selectedItemConfig}
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
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: paths.GetDpPath(selectedItemConfig.Path, fileName), Parent: selectedItemConfig}
			item := model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
			items = append(items, item)
		}

		sort.Sort(items)
		return items, nil
	} else if config.DpUseSoma() {
		dpFilestoreLocation, _ := splitOnFirst(selectedItemConfig.Path, "/")
		dpFilestoreIsRoot := !strings.Contains(selectedItemConfig.Path, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node

		// If we open filestore or open file but want to reload - refresh current filestore XML cache.
		if dpFilestoreIsRoot || r.invalidateCache {
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItemConfig.DpDomain + "\">" +
				"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\" location=\"" + dpFilestoreLocation + "\"/></dp:request>" +
				"</soapenv:Body></soapenv:Envelope>"
			var err error
			r.dpFilestoreXml, err = dpnet.Soma(somaRequest)
			if err != nil {
				return nil, err
			}
			r.invalidateCache = false
		}

		if dpFilestoreIsRoot {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogDebug("Error parsing response JSON.", err)
				return nil, err
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file")
			// println(dpFilestoreLocation)
		} else {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogFatal(err)
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
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: dirFullName, Parent: selectedItemConfig}
			// Path: selectedItemConfig.Path
			items[idx] = model.Item{Name: dirName, Config: &itemConfig}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: selectedItemConfig.Path, Parent: selectedItemConfig}
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

func findItemConfigParentDomain(itemConfig *model.ItemConfig) *model.ItemConfig {
	if itemConfig.Type == model.ItemDpDomain {
		return itemConfig
	}
	if itemConfig.Parent == nil {
		return nil
	}
	return findItemConfigParentDomain(itemConfig.Parent)
}

func fetchDpDomains() ([]string, error) {
	logging.LogDebug("repo/dp/fetchDpDomains()")
	domains := make([]string, 0)

	if config.DpUseRest() {
		bodyString, err := dpnet.RestGet("/mgmt/domains/config/")
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
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
			"<soapenv:Body><dp:GetDomainListRequest xmlns:dp=\"http://www.datapower.com/schemas/appliance/management/1.0\"/></soapenv:Body>" +
			"</soapenv:Envelope>"
		somaResponse, err := dpnet.Amp(somaRequest)
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
