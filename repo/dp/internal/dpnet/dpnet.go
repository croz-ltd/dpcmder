package dpnet

import (
	"crypto/tls"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// InitNetworkSettings initializes TLS & proxy configuration.
func InitNetworkSettings() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if *config.Proxy != "" {
		proxyUrl, err := url.Parse(*config.Proxy)
		if err != nil {
			logging.LogFatal("Couldn't initialize network settings to access DataPower.", err)
		}
		http.DefaultTransport.(*http.Transport).Proxy = http.ProxyURL(proxyUrl)
	}
}

// rest makes http request from relative URL path given, method and body.
func Rest(urlPath, method, body string) (string, error) {
	fullURL := *config.DpRestURL + urlPath
	return httpRequest(fullURL, method, body)
}

// RestGet makes DataPower REST GET request.
func RestGet(urlPath string) (string, error) {
	return Rest(urlPath, "GET", "")
}

// Amp makes DataPower AMP request.
func Amp(body string) (string, error) {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/amp/1.0", "POST", body)
}

// Soma makes DataPower SOMA request.
func Soma(body string) (string, error) {
	return httpRequest(*config.DpSomaURL+"/service/mgmt/current", "POST", body)
}

// httpRequest makes DataPower HTTP request.
func httpRequest(urlFullPath, method, body string) (string, error) {
	logging.LogTracef("repo/dp/dpnet/httpRequest(%s, %s, '%s')", urlFullPath, method, body)

	client := &http.Client{}
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlFullPath, bodyReader)
	if err != nil {
		logging.LogDebug("repo/dp/dpnet/httpRequest() - Can't prepare request: ", err)
		return "", err
	}

	// logging.LogDebugf("dp username:password: '%s:%s'", *config.DpUsername, config.DpPassword())
	req.SetBasicAuth(*config.DpUsername, config.DpPassword())
	resp, err := client.Do(req)

	if err != nil {
		logging.LogDebug("repo/dp/dpnet/httpRequest() - Can't send request: ", err)
		return "", err
		// 2019/10/22 08:39:14 dp Can't send request: Post https://10.123.56.55:5550/service/mgmt/current: dial tcp 10.123.56.55:5550: i/o timeout
		//exit status 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logging.LogDebug("repo/dp/dpnet/httpRequest() - Can't read response: ", err)
			return "", err
		}
		logging.LogTracef("repo/dp/dpnet/httpRequest() - httpResponse: '%s'", string(bodyBytes))
		return string(bodyBytes), nil
	}
	// logging.LogTracef("repo/dp/dpnet/httpRequest() - resp.StatusCode: '%d'", resp.StatusCode)
	// if resp.StatusCode == 403 || resp.StatusCode == 404 {
	// 	return ""
	// }
	logging.LogDebugf("repo/dp/dpnet/httpRequest() - HTTP %s call to '%s' returned HTTP StatusCode %v (%s)",
		method, urlFullPath, resp.StatusCode, resp.Status)
	return "", errs.UnexpectedHTTPResponse{StatusCode: resp.StatusCode, Status: resp.Status}
}

// MakeRestPath creates DataPower REST path to given domain.
func MakeRestPath(dpDomain, filePath string) string {
	logging.LogDebugf("repo/dp/dpnet/MakeRestPath('%s', '%s')", dpDomain, filePath)
	currRestFilePath := strings.Replace(filePath, ":", "", 1)
	return "/mgmt/filestore/" + dpDomain + "/" + currRestFilePath
}
