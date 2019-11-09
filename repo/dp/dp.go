package dp

import (
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp/internal/dpnet"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"sort"
	"strings"
)

type DpRepo struct {
	name           string
	dpFilestoreXml string
}

var Repo = DpRepo{name: "DpRepo"}

func InitNetworkSettings() {
	dpnet.InitNetworkSettings()
}

func (r *DpRepo) GetInitialView() model.CurrentView {
	initialView := model.CurrentView{
		Type:        model.ItemNone,
		Path:        "",
		DpAppliance: "",
		DpDomain:    ""}
	return initialView
}

func (r *DpRepo) GetTitle(view model.CurrentView) string {
	dpDomain := view.DpDomain
	currPath := view.Path

	var url *string
	if *config.DpRestURL != "" {
		url = config.DpRestURL
	} else {
		url = config.DpSomaURL
	}

	return fmt.Sprintf("%s @ %s (%s) %s", *config.DpUsername, *url, dpDomain, currPath)
}
func (r *DpRepo) GetList(currentView model.CurrentView) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/GetList(%v)", currentView))

	if currentView.DpAppliance == "" {
		return r.listAppliances()
	} else if currentView.DpDomain == "" {
		return r.listDomains()
	} else if currentView.Path == "" {
		return r.listFilestores(currentView.DpDomain)
	} else {
		return r.listDpDir(currentView.DpDomain, currentView.Path)
	}
}

func (r *DpRepo) NextView(currView model.CurrentView, selectedItem model.Item) model.CurrentView {
	return model.CurrentView{}
}

// listAppliances returns ItemList of DataPower appliance Items from configuration.
func (r *DpRepo) listAppliances() model.ItemList {
	appliances := config.Conf.DataPowerAppliances
	logging.LogDebug(fmt.Sprintf("repo/dp/listAppliances(), appliances: %v", appliances))

	items := make(model.ItemList, len(appliances))
	idx := 0
	for name := range appliances {
		items[idx] = model.Item{Type: model.ItemDpConfiguration, Name: name, Size: "", Modified: "", Selected: false}
		idx = idx + 1
	}

	sort.Sort(items)

	return items
}

// listDomains loads DataPower domains from current DataPower.
func (r *DpRepo) listDomains() model.ItemList {
	logging.LogDebug("repo/dp/listDomains()")
	domainNames := fetchDpDomains()
	logging.LogDebug(fmt.Sprintf("repo/dp/listDomains(), domainNames: %v", domainNames))

	items := make(model.ItemList, len(domainNames)+1)
	items[0] = model.Item{Type: model.ItemDpConfiguration, Name: "..", Size: "", Modified: "", Selected: false}

	for idx, name := range domainNames {
		items[idx+1] = model.Item{Type: model.ItemDpDomain, Name: name, Size: "", Modified: "", Selected: false}
	}

	sort.Sort(items)

	return items
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *DpRepo) listFilestores(dpDomain string) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/listFilestores('%s')", dpDomain))
	if config.DpUseRest() {
		jsonString := dpnet.RestGet("/mgmt/filestore/" + dpDomain)
		// println("jsonString: " + jsonString)

		// .filestore.location[]?.name
		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := jsonquery.Find(doc, "/filestore/location/*/name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Type: model.ItemDpDomain, Name: "..", Size: "", Modified: "", Selected: false}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			items[idx+1] = model.Item{Type: model.ItemDpFilestore, Name: filestoreName, Size: "", Modified: "", Selected: false}
		}

		sort.Sort(items)

		return items
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + dpDomain + "\">" +
			"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		r.dpFilestoreXml = dpnet.Soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := xmlquery.Find(doc, "//*[local-name()='location']/@name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Type: model.ItemDpDomain, Name: "..", Size: "", Modified: "", Selected: false}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			items[idx+1] = model.Item{Type: model.ItemDpFilestore, Name: filestoreName, Size: "", Modified: "", Selected: false}
		}

		sort.Sort(items)

		return items
	}

	logging.LogFatal("repo/dp/listFilestores(), unknown Dp management interface.")
	return nil
}

// listDpDir loads DataPower directory (local:, local:///test,..).
func (r *DpRepo) listDpDir(dpDomain string, currPath string) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/listDpDir('%s', '%s')", dpDomain, currPath))
	parentDir := model.Item{Type: model.ItemDirectory, Name: "..", Size: "", Modified: "", Selected: false}
	filesDirs := r.listFiles(dpDomain, currPath)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, filesDirs...)

	return itemsWithParentDir
}

func (r *DpRepo) listFiles(dpDomain string, dirPath string) []model.Item {
	filesDirs := make(model.ItemList, 0)

	if config.DpUseRest() {
		currRestDirPath := strings.Replace(dirPath, ":", "", 1)
		jsonString := dpnet.RestGet("/mgmt/filestore/" + dpDomain + "/" + currRestDirPath)
		// println("jsonString: " + jsonString)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}

		// .filestore.location.directory /name
		// work-around - for one directory we get JSON object, for multiple directories we get JSON array
		fileNodes := jsonquery.Find(doc, "/filestore/location/directory//name/..")
		for _, n := range fileNodes {
			dirDpPath := n.SelectElement("name").InnerText()
			_, dirName := utils.SplitOnLast(dirDpPath, "/")
			item := model.Item{Type: model.ItemDirectory, Name: dirName, Size: "", Modified: "", Selected: false}
			filesDirs = append(filesDirs, item)
		}

		// .filestore.location.file      /name, /size, /modified
		dirNodes := jsonquery.Find(doc, "/filestore/location/file//name/..")
		for _, n := range dirNodes {
			fileName := n.SelectElement("name").InnerText()
			fileSize := n.SelectElement("size").InnerText()
			fileModified := n.SelectElement("modified").InnerText()
			item := model.Item{Type: model.ItemFile, Name: fileName, Size: fileSize, Modified: fileModified, Selected: false}
			filesDirs = append(filesDirs, item)
		}
	} else if config.DpUseSoma() {
		dpFilestoreLocation, _ := utils.SplitOnFirst(dirPath, "/")
		dpFilestoreIsRoot := !strings.Contains(dirPath, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node
		if dpFilestoreIsRoot {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogFatal(err)
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file")
			// println(dpFilestoreLocation)
		} else {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogFatal(err)
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+dirPath+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+dirPath+"']/file")
		}

		dirNum := len(dpDirNodes)
		items := make(model.ItemList, dirNum+len(dpFileNodes))
		for idx, node := range dpDirNodes {
			// "local:"
			dirFullName := node.SelectAttr("name")
			_, dirName := utils.SplitOnLast(dirFullName, "/")
			items[idx] = model.Item{Type: model.ItemDirectory, Name: dirName, Size: "", Modified: "", Selected: false}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			items[idx+dirNum] = model.Item{Type: model.ItemFile, Name: fileName, Size: fileSize, Modified: fileModified, Selected: false}
		}

		return items
	}

	sort.Sort(filesDirs)

	return filesDirs
}

func fetchDpDomains() []string {
	logging.LogDebug(fmt.Sprintf("repo/dp/fetchDpDomains()"))
	domains := make([]string, 0)

	if config.DpUseRest() {
		bodyString := dpnet.RestGet("/mgmt/domains/config/")

		// .domain[].name
		doc, err := jsonquery.Parse(strings.NewReader(bodyString))
		if err != nil {
			logging.LogFatal(err)
		}
		list := jsonquery.Find(doc, "/domain//name")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
			"<soapenv:Body><dp:GetDomainListRequest xmlns:dp=\"http://www.datapower.com/schemas/appliance/management/1.0\"/></soapenv:Body>" +
			"</soapenv:Envelope>"
		somaResponse := dpnet.Amp(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogFatal(err)
		}
		list := xmlquery.Find(doc, "//*[local-name()='GetDomainListResponse']/*[local-name()='Domain']/text()")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	}

	return domains
}
