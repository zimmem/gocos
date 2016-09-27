package cosclient

/**
 * CosClient
 */
type CosClient struct {
	appID     int
	secretID  string
	secretKey string
	bucket    string
}

func (c *CosClient) upload(local string, remote string) {

}

func (c *CosClient) download(remote string, local string) {

}

func (client *CosClient) list(path string) {

}

func (client *CosClient) deleteFile(path string) {

}

func (client *CosClient) deleteDirectory(path string) {

}
