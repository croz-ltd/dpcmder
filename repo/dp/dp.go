package dp

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	dpSide = model.Left
)

type DpRepo struct {
	name           string
	dpFilestoreXml string
}

var Repo = DpRepo{name: "DpRepo"}

func rest(urlPath, method, body string) string {
	fullUrl := *config.DpRestURL + urlPath
	return httpRequest(fullUrl, method, body)
}

func restGet(urlPath string) string {
	return rest(urlPath, "GET", "")
}

func amp(body string) string {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/amp/1.0", "POST", body)
}

func soma(body string) string {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/current", "POST", body)
}

func InitNetworkSettings() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if *config.Proxy != "" {
		proxyUrl, err := url.Parse(*config.Proxy)
		if err != nil {
			logging.LogFatal(err)
		}
		http.DefaultTransport.(*http.Transport).Proxy = http.ProxyURL(proxyUrl)
	}
}

func httpRequest(urlFullPath, method, body string) string {
	logging.LogDebug(fmt.Sprintf("dp.httpRequest(%s, %s, '%s')", urlFullPath, method, body))

	client := &http.Client{}
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlFullPath, bodyReader)
	if err != nil {
		logging.LogFatal(err)
	}

	req.SetBasicAuth(*config.DpUsername, *config.DpPassword)
	resp, err := client.Do(req)

	if err != nil {
		logging.LogFatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		bodyBytes, err2 := ioutil.ReadAll(resp.Body)
		if err2 != nil {
			logging.LogFatal(err2)
		}
		logging.LogDebug(fmt.Sprintf("dp httpResponse: '%s'", string(bodyBytes)))
		return string(bodyBytes)
	} else {
		if resp.StatusCode == 404 {
			return ""
		}
		logging.LogFatal("HTTP " + method + " call to '" + urlFullPath + "' returned HTTP StatusCode " + string(resp.StatusCode) + "(" + resp.Status + ")")
		return ""
	}
}

func fetchDpDomains() []string {
	domains := make([]string, 0)

	if config.DpUseRest() {
		bodyString := restGet("/mgmt/domains/config/")

		// .domain[].name
		doc, err := jsonquery.Parse(strings.NewReader(bodyString))
		if err != nil {
			logging.LogFatal(err)
		}
		list := jsonquery.Find(doc, "/domain/*/name")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
			"<soapenv:Body><dp:GetDomainListRequest xmlns:dp=\"http://www.datapower.com/schemas/appliance/management/1.0\"/></soapenv:Body>" +
			"</soapenv:Envelope>"
		somaResponse := amp(somaRequest)
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

func (r *DpRepo) InitialLoad(m *model.Model) {
	logging.LogDebug(fmt.Sprintf("InitialLoad(), m.DpDomain(): %s", m.DpDomain()))

	m.SetCurrPathForSide(dpSide, "")
	setTitle(m, "")

	r.LoadCurrent(m)
}

func (r *DpRepo) LoadCurrent(m *model.Model) {
	logging.LogDebug(fmt.Sprintf("LoadCurrent(), dpSide: %v", dpSide))

	currPath := m.CurrPathForSide(dpSide)
	if m.DpDomain() == "" {
		r.loadDomains(m)
	} else if currPath == "" {
		r.loadFilestores(m)
	} else {
		r.loadCurrentPath(m)
	}
}

func setTitle(m *model.Model, currPath string) {
	var url *string
	if *config.DpRestURL != "" {
		url = config.DpRestURL
	} else {
		url = config.DpSomaURL
	}

	m.SetTitle(dpSide, fmt.Sprintf("%s @ %s (%s) %s", *config.DpUsername, *url, m.DpDomain(), currPath))
}

func (r *DpRepo) EnterCurrentDirectory(m *model.Model) {
	dpDomain := m.DpDomain()
	currPath := m.CurrPathForSide(dpSide)
	logging.LogDebug("dp.EnterCurrentDirectory() BEGIN currPath: " + currPath + "\n")
	dirName := m.CurrItemForSide(dpSide).Name
	newCurrentItemName := ".."
	if dpDomain == "" {
		m.SetDpDomain(m.CurrItemForSide(dpSide).Name)
	} else if dirName == ".." {
		if currPath == "" {
			m.SetDpDomain("")
		} else if strings.HasSuffix(currPath, ":") {
			currPath, newCurrentItemName = "", currPath
		} else {
			currPath, newCurrentItemName = utils.SplitOnLast(currPath, "/")
		}
	} else {
		if currPath != "" {
			currPath += "/" + dirName
		} else {
			currPath = dirName
		}
	}

	m.SetCurrPathForSide(dpSide, currPath)
	r.loadCurrentPath(m)
	m.SetCurrItemForSide(dpSide, newCurrentItemName)
}

func (r *DpRepo) ListFiles(m *model.Model, dirPath string) []model.Item {
	filesDirs := make(model.ItemList, 0)

	if config.DpUseRest() {
		currRestDirPath := strings.Replace(dirPath, ":", "", 1)
		jsonString := restGet("/mgmt/filestore/" + m.DpDomain() + "/" + currRestDirPath)
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
			item := model.Item{Type: 'd', Name: dirName, Size: "", Modified: "", Selected: false}
			filesDirs = append(filesDirs, item)
		}

		// .filestore.location.file      /name, /size, /modified
		dirNodes := jsonquery.Find(doc, "/filestore/location/file//name/..")
		for _, n := range dirNodes {
			fileName := n.SelectElement("name").InnerText()
			fileSize := n.SelectElement("size").InnerText()
			fileModified := n.SelectElement("modified").InnerText()
			item := model.Item{Type: 'f', Name: fileName, Size: fileSize, Modified: fileModified, Selected: false}
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
			items[idx] = model.Item{Type: 'd', Name: dirName, Size: "", Modified: "", Selected: false}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			items[idx+dirNum] = model.Item{Type: 'f', Name: fileName, Size: fileSize, Modified: fileModified, Selected: false}
		}

		return items
	}

	sort.Sort(filesDirs)

	return filesDirs
}

func (r *DpRepo) GetFileName(filePath string) string {
	lastSlashIdx := strings.LastIndex(filePath, "/")
	if lastSlashIdx != -1 && len(filePath) > 1 {
		return filePath[lastSlashIdx+1:]
	} else {
		return ""
	}
}

func (r *DpRepo) GetFilePath(parentPath, relativePath string) string {
	logging.LogDebug(fmt.Sprintf("dp.GetFilePath('%s', '%s')\n", parentPath, relativePath))
	if relativePath == ".." {
		lastSlashIdx := strings.LastIndex(parentPath, "/")
		if lastSlashIdx != -1 && len(parentPath) > 1 {
			return parentPath[:lastSlashIdx]
		} else {
			return parentPath
		}
	} else if relativePath == "" {
		return parentPath
	} else {
		// For case we get relativPath from Windwos local fs
		return parentPath + "/" + strings.Replace(relativePath, "\\", "/", -1)
	}
}

func (r *DpRepo) GetFileTypeFromPath(m *model.Model, filePath string) byte {
	logging.LogDebug(fmt.Sprintf("dp.GetFileTypeFromPath('%s')\n", filePath))
	parentPath, fileName := utils.SplitOnLast(filePath, "/")
	return r.GetFileType(m, parentPath, fileName)
}

func (r *DpRepo) GetFileType(m *model.Model, parentPath, fileName string) byte {
	filePath := r.GetFilePath(parentPath, fileName)
	logging.LogDebug(fmt.Sprintf("dp.GetFileType('%s', '%s')\n", parentPath, fileName))

	if config.DpUseRest() {
		restPath := r.makeRestPath(m, filePath)
		jsonString := restGet(restPath)
		// println("jsonString: " + jsonString)

		if jsonString == "" {
			return '0'
		}

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal("repo.dp.GetFileType()", err)
		}

		filestore := jsonquery.Find(doc, "/filestore")
		file := jsonquery.Find(doc, "/file")
		if len(filestore) == 1 {
			return 'd'
		} else if len(file) == 1 {
			return 'f'
		}

		logging.LogFatal(fmt.Sprintf("ERROR: repo.dp.GetFileType() - wronge response: '%s'", jsonString))
	} else if config.DpUseSoma() {
		if parentPath != "" {
			dpFilestoreLocation, _ := utils.SplitOnFirst(parentPath, "/")
			dpFilestoreIsRoot := !strings.Contains(parentPath, "/")
			var dpDirNodes []*xmlquery.Node
			var dpFileNodes []*xmlquery.Node
			if dpFilestoreIsRoot {
				doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
				if err != nil {
					logging.LogFatal(err)
				}
				dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory[@name='"+filePath+"']")
				dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file[@name='"+fileName+"']")
				// println(dpFilestoreLocation)
			} else {
				doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
				if err != nil {
					logging.LogFatal(err)
				}
				dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+filePath+"']")
				dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+parentPath+"']/file[@name='"+fileName+"']")
			}

			if len(dpDirNodes) == 1 {
				return 'd'
			} else if len(dpFileNodes) == 1 {
				return 'f'
			}
		} else {
			if m.DpDomain() != "" {
				return 'd'
			}
		}
	}

	return '0'
}

func (r *DpRepo) GetFileByPath(m *model.Model, dpPath string) []byte {
	parentPath, fileName := utils.SplitOnLast(dpPath, "/")
	return r.GetFile(m, parentPath, fileName)
}
func (r *DpRepo) GetFile(m *model.Model, parentPath, fileName string) []byte {
	filePath := r.GetFilePath(parentPath, fileName)

	if config.DpUseRest() {
		restPath := r.makeRestPath(m, filePath)
		jsonString := restGet(restPath)
		// println("jsonString: " + jsonString)

		if jsonString == "" {
			return nil
		} else {
			doc, err := jsonquery.Parse(strings.NewReader(jsonString))
			if err != nil {
				logging.LogFatal("repo.dp.GetFile(), err: ", err)
			}

			// .filestore.location.directory /name
			// work-around - for one directory we get JSON object, for multiple directories we get JSON array
			fileNode := jsonquery.FindOne(doc, "/file")
			if fileNode == nil {
				return nil
				// logging.LogFatal(fmt.Sprintf("Expected file '%s' not found.", filePath))
			} else {
				fileB64 := fileNode.InnerText()
				resultBytes, err := base64.StdEncoding.DecodeString(fileB64)
				if err != nil {
					logging.LogFatal("repo.dp.GetFile()", err)
				}

				return resultBytes
			}
		}
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
			"<dp:get-file name=\"" + filePath + "\"/></dp:request></soapenv:Body></soapenv:Envelope>"
		somaResponse := soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogFatal(err)
		}
		fileNode := xmlquery.FindOne(doc, "//*[local-name()='file']")

		if fileNode == nil {
			return nil
			// logging.LogFatal(fmt.Sprintf("Expected file '%s' not found.", filePath))
		} else {
			fileB64 := fileNode.InnerText()
			resultBytes, err := base64.StdEncoding.DecodeString(fileB64)
			if err != nil {
				logging.LogFatal("repo.dp.GetFile()", err)
			}

			return resultBytes
		}
	}

	return nil
}

func (r *DpRepo) UpdateFileByPath(m *model.Model, dpPath string, newFileContent []byte) bool {
	parentPath, fileName := utils.SplitOnLast(dpPath, "/")
	return r.UpdateFile(m, parentPath, fileName, newFileContent)
}
func (r *DpRepo) UpdateFile(m *model.Model, parentPath, fileName string, newFileContent []byte) bool {
	logging.LogDebug(fmt.Sprintf("dp.UpdateFile(%s, %s)\n", parentPath, fileName))
	fileType := r.GetFileType(m, parentPath, fileName)

	if config.DpUseRest() {
		updateFilePath := parentPath
		restMethod := "POST"
		if fileType == 'f' {
			updateFilePath = r.GetFilePath(parentPath, fileName)
			restMethod = "PUT"
		} else if fileType == 'd' {
			logging.LogFatal(fmt.Sprintf("ERROR: can't upload file '%s' to '%s', directory with same name exists.", fileName, parentPath))
		}
		requestBody := "{\"file\":{\"name\":\"" + fileName + "\",\"content\":\"" + base64.StdEncoding.EncodeToString(newFileContent) + "\"}}"

		restPath := r.makeRestPath(m, updateFilePath)
		jsonString := rest(restPath, restMethod, requestBody)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal("repo.dp.UpdateFile()", err)
		}

		jsonError := jsonquery.Find(doc, "/error")
		if len(jsonError) != 0 {
			logging.LogFatal(fmt.Sprintf("ERROR uploading file '%s' to '%s', returned '%s'.", fileName, parentPath, jsonString))
		}

		return true
	} else if config.DpUseSoma() {
		if fileType == 'd' {
			logging.LogFatal(fmt.Sprintf("ERROR: can't upload file '%s' to '%s', directory with same name exists.", fileName, parentPath))
		} else {
			filePath := r.GetFilePath(parentPath, fileName)
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
				"<dp:set-file name=\"" + filePath + "\">" + base64.StdEncoding.EncodeToString(newFileContent) + "</dp:set-file>" +
				"</dp:request></soapenv:Body></soapenv:Envelope>"
			somaResponse := soma(somaRequest)
			doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
			if err != nil {
				logging.LogFatal(err)
			}
			r.loadFilestores(m)
			resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
			if resultNode != nil {
				resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
				if resultText == "OK" {
					return true
				}
			}
		}
	}

	return false
}
func (r *DpRepo) Delete(m *model.Model, parentPath, fileName string) bool {
	filePath := r.GetFilePath(parentPath, fileName)

	if config.DpUseRest() {
		restPath := r.makeRestPath(m, filePath)
		jsonString := rest(restPath, "DELETE", "")
		// println("jsonString: " + jsonString)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal("repo.dp.Delete()", err)
		}

		error := jsonquery.Find(doc, "/error")
		if len(error) == 0 {
			return true
		}
	} else if config.DpUseSoma() {
		fileType := r.GetFileType(m, parentPath, fileName)
		var somaRequest string
		if fileType == 'd' {
			somaRequest = "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
				"<dp:do-action><RemoveDir><Dir>" + filePath + "</Dir></RemoveDir></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
		} else {
			somaRequest = "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
				"<dp:do-action><DeleteFile><File>" + filePath + "</File></DeleteFile></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
		}
		somaResponse := soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogFatal(err)
		}
		r.loadFilestores(m)
		resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
		if resultNode != nil {
			resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
			if resultText == "OK" {
				return true
			}
		}
	}

	return false
}

func (r *DpRepo) CreateDirByPath(m *model.Model, dirPath string) bool {
	parentPath, dirName := utils.SplitOnLast(dirPath, "/")
	return r.CreateDir(m, parentPath, dirName)
}
func (r *DpRepo) CreateDir(m *model.Model, parentPath, dirName string) bool {
	fileType := r.GetFileType(m, parentPath, dirName)
	logging.LogDebug(fmt.Sprintf("dp.CreateDir(m, ''%s', ''%s')", parentPath, dirName))

	if fileType == '0' {
		if config.DpUseRest() {
			requestBody := "{\"directory\":{\"name\":\"" + dirName + "\"}}"
			restPath := r.makeRestPath(m, parentPath)
			jsonString := rest(restPath, "POST", requestBody)
			// println("jsonString: " + jsonString)

			doc, err := jsonquery.Parse(strings.NewReader(jsonString))
			if err != nil {
				logging.LogFatal(fmt.Sprintf("repo.dp.CreateDir('%s', '%s')", parentPath, dirName), err)
			}

			error := jsonquery.Find(doc, "/error")
			if len(error) == 0 {
				return true
			} else {
				logging.LogFatal(fmt.Sprintf("ERROR: can't create dir '%s' at '%s', json returned : '%s'.", dirName, parentPath, jsonString))
			}
		} else if config.DpUseSoma() {
			dirPath := r.GetFilePath(parentPath, dirName)
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
				"<dp:do-action><CreateDir><Dir>" + dirPath + "</Dir></CreateDir></dp:do-action></dp:request></soapenv:Body></soapenv:Envelope>"
			somaResponse := soma(somaRequest)
			doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
			if err != nil {
				logging.LogFatal(err)
			}
			r.loadFilestores(m)
			resultNode := xmlquery.FindOne(doc, "//*[local-name()='response']/*[local-name()='result']")
			if resultNode != nil {
				resultText := strings.Trim(resultNode.InnerText(), " \n\r\t")
				if resultText == "OK" {
					return true
				}
			}
		}
	} else if fileType == 'f' {
		logging.LogFatal(fmt.Sprintf("ERROR: can't create dir '%s' at '%s', file with same name exists.", dirName, parentPath))
	}

	return false
}

func (r *DpRepo) IsEmptyDir(m *model.Model, parentPath, dirName string) bool {
	dirPath := r.GetFilePath(parentPath, dirName)
	subItems := r.ListFiles(m, dirPath)

	return len(subItems) == 0
}

func (r *DpRepo) makeRestPath(m *model.Model, filePath string) string {
	currRestFilePath := strings.Replace(filePath, ":", "", 1)
	return "/mgmt/filestore/" + m.DpDomain() + "/" + currRestFilePath
}

// loadDomains loads DataPower domains from current DataPower.
func (r *DpRepo) loadDomains(m *model.Model) {
	domainNames := fetchDpDomains()
	logging.LogDebug(fmt.Sprintf("loadDomains(), domainNames: %v", domainNames))

	items := make(model.ItemList, len(domainNames))

	for idx, name := range domainNames {
		items[idx] = model.Item{Type: 'd', Name: name, Size: "", Modified: "", Selected: false}
	}

	sort.Sort(items)

	m.SetItems(dpSide, items)
}

// loadFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *DpRepo) loadFilestores(m *model.Model) {
	if config.DpUseRest() {
		jsonString := restGet("/mgmt/filestore/" + m.DpDomain())
		// println("jsonString: " + jsonString)

		// .filestore.location[]?.name
		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := jsonquery.Find(doc, "/filestore/location/*/name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Type: 'd', Name: "..", Size: "", Modified: "", Selected: false}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			items[idx+1] = model.Item{Type: 'd', Name: filestoreName, Size: "", Modified: "", Selected: false}
		}

		sort.Sort(items)

		m.SetItems(dpSide, items)
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + m.DpDomain() + "\">" +
			"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		r.dpFilestoreXml = soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := xmlquery.Find(doc, "//*[local-name()='location']/@name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Type: 'd', Name: "..", Size: "", Modified: "", Selected: false}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			items[idx+1] = model.Item{Type: 'd', Name: filestoreName, Size: "", Modified: "", Selected: false}
		}

		sort.Sort(items)

		m.SetItems(dpSide, items)

		// list := xmlquery.Find(doc, "//*[local-name()='GetDomainListResponse']/*[local-name()='Domain']/text()")
		// for _, n := range list {
		// 	domains = append(domains, n.InnerText())
		// }
	}
}

// loadDpDir loads DataPower directory (local:, local:///test,..).
func (r *DpRepo) loadDpDir(m *model.Model, currPath string) {
	parentDir := model.Item{Type: 'd', Name: "..", Size: "", Modified: "", Selected: false}
	filesDirs := r.ListFiles(m, currPath)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, filesDirs...)

	m.SetItems(dpSide, itemsWithParentDir)
}

func (r *DpRepo) loadCurrentPath(m *model.Model) {
	dpDomain := m.DpDomain()
	currPath := m.CurrPathForSide(dpSide)
	setTitle(m, currPath)

	if dpDomain == "" {
		r.loadDomains(m)
	} else if currPath == "" {
		r.loadFilestores(m)
	} else {
		r.loadDpDir(m, currPath)
	}
}
