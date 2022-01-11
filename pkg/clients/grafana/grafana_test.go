package grafana

//import (
//	"fmt"
//	"net/url"
//	"os"
//	"testing"
//
//	"k8s.io/klog/v2"
//
//	grafanaclient "github.com/grafana/grafana-api-golang-client"
//)
//
//func TestClient(t *testing.T) {
//	gc, err := grafanaclient.New("http://110.40.196.157:31112/", grafanaclient.Config{
//		BasicAuth: url.UserPassword("admin", "admin"),
//	})
//	if err != nil {
//		klog.Fatal(err)
//	}
//	dashboards, err := gc.Dashboards()
//	if err != nil {
//		fmt.Println(err)
//		os.Exit(-1)
//	}
//	for _, dashboard := range dashboards {
//		fmt.Println(dashboard)
//	}
//}
