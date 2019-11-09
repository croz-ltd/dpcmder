package dpnet

import (
	"crypto/tls"
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

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

func rest(urlPath, method, body string) string {
	fullURL := *config.DpRestURL + urlPath
	return httpRequest(fullURL, method, body)
}

func RestGet(urlPath string) string {
	return rest(urlPath, "GET", "")
}

func Amp(body string) string {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/amp/1.0", "POST", body)
}

func Soma(body string) string {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/current", "POST", body)
}

func httpRequest(urlFullPath, method, body string) string {
	logging.LogDebug(fmt.Sprintf("repo/dp/httpRequest(%s, %s, '%s')", urlFullPath, method, body))

	client := &http.Client{}
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlFullPath, bodyReader)
	if err != nil {
		logging.LogFatal("repo/dp/httpRequest() - Can't prepare request: ", err)
	}

	// logging.LogDebug(fmt.Sprintf("dp username:password: '%s:%s'", *config.DpUsername, config.DpPassword()))
	req.SetBasicAuth(*config.DpUsername, config.DpPassword())
	resp, err := client.Do(req)

	if err != nil {
		logging.LogFatal("repo/dp/httpRequest() - Can't send request: ", err)
		// 2019/10/22 08:39:14 dp Can't send request: Post https://10.123.56.55:5550/service/mgmt/current: dial tcp 10.123.56.55:5550: i/o timeout
		//exit status 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logging.LogFatal("repo/dp/httpRequest() - Can't read response: ", err)
		}
		logging.LogDebug(fmt.Sprintf("repo/dp/httpRequest() - httpResponse: '%s'", string(bodyBytes)))
		return string(bodyBytes)
	} else {
		logging.LogDebug(fmt.Sprintf("repo/dp/httpRequest() - resp.StatusCode: '%d'", resp.StatusCode))
		if resp.StatusCode == 403 || resp.StatusCode == 404 {
			return ""
		}
		logging.LogFatal(fmt.Sprintf("repo/dp/httpRequest() - HTTP %s call to '%s' returned HTTP StatusCode %v (%s)",
			method, urlFullPath, resp.StatusCode, resp.Status))
		return ""
	}
}

func makeRestPath(dpDomain, filePath string) string {
	currRestFilePath := strings.Replace(filePath, ":", "", 1)
	return "/mgmt/filestore/" + dpDomain + "/" + currRestFilePath
}
