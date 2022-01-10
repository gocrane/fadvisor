package cvm

//import (
//	"encoding/json"
//	"fmt"
//	"os"
//	"testing"
//	"time"
//
//	"k8s.io/client-go/util/flowcontrol"
//
//	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
//	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/consts"
//	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
//
//	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
//	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
//)
//
//func GetTencentCloudAccessKey() (id, secret string) {
//	return os.Getenv("TENCENTCLOUD_ACCESS_KEY_ID"),
//		os.Getenv("TENCENTCLOUD_SECRET_ACCESS_KEY")
//}
//
//func TestInstancePrice(t *testing.T) {
//	//ins-inlfr4as
//
//	id, key := GetTencentCloudAccessKey()
//	qccp := qcloud.QCloudClientProfile{
//		Debug:           true,
//		DefaultLanguage: consts.LANGUAGE,
//		DefaultLimit:    consts.LIMITS,
//		DefaultTimeout:  consts.TIMEOUT,
//		Region:          "ap-shenzhen",
//		DomainSuffix:    "internal.tencentcloudapi.com",
//		Scheme:          "",
//	}
//
//	cred := credential.NewQCloudCredential("", "", id, key, 1*time.Hour)
//	qcc := &qcloud.QCloudClientConfig{
//		RateLimiter:         flowcontrol.NewTokenBucketRateLimiter(5, 1),
//		DefaultRetryCnt:     consts.MAXRETRY,
//		QCloudClientProfile: qccp,
//		Credential:          cred,
//	}
//
//	cvmClient := NewCVMClient(qcc)
//	//insList := []string{"ins-inlfr4as", "ins-ni89g600", "ins-07x3i52c", "ins-43ksg2z2", "ins-dmecim98", "ins-33110ldy"}
//	insList := []string{"ins-bmju196s", "ins-7cq51qme", "ins-kz0rb05c", "ins-2bwd971g", "ins-gcj6asry", "ins-iw9cecg4", "ins-36h9tyfc"}
//	insListPtrs := []*string{}
//	for i := range insList {
//		insListPtrs = append(insListPtrs, &insList[i])
//	}
//	instances, err := cvmClient.GetCVMInstances(insListPtrs)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	items, err := cvmClient.GetAllZoneInstanceConfigInfos()
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	standardPricings := make(map[string]*cvm.InstanceTypeQuotaItem)
//	for _, item := range items {
//		zone := *item.Zone
//		insType := *item.InstanceType
//		insChargeType := *item.InstanceChargeType
//		key := zone + "," + insType + "," + insChargeType
//		standardPricings[key] = item
//	}
//	fmt.Println(len(items))
//	fmt.Println(len(standardPricings))
//	for _, ins := range instances {
//		zone := ""
//		insType := ""
//		insChargeType := ""
//		if ins.Placement != nil && ins.Placement.Zone != nil {
//			zone = *ins.Placement.Zone
//		}
//		if ins.InstanceType != nil {
//			insType = *ins.InstanceType
//		}
//		if ins.InstanceChargeType != nil {
//			insChargeType = *ins.InstanceChargeType
//		}
//
//		req := cvm.NewDescribeZoneInstanceConfigInfosRequest()
//
//		req.Filters = []*cvm.Filter{
//			{
//				Name:   common.StringPtr("instance-type"),
//				Values: common.StringPtrs([]string{insType}),
//			},
//			{
//				Name:   common.StringPtr("instance-charge-type"),
//				Values: common.StringPtrs([]string{insChargeType}),
//			},
//			{
//				Name:   common.StringPtr("zone"),
//				Values: common.StringPtrs([]string{zone}),
//			},
//		}
//		resp, err := cvmClient.client.DescribeZoneInstanceConfigInfos(req)
//		if err != nil {
//			t.Fatal(err)
//		}
//		fmt.Println(*resp.Response.RequestId)
//		if resp.Response != nil {
//			for _, item := range resp.Response.InstanceTypeQuotaSet {
//				out, err := json.MarshalIndent(item.Price, "", "  ")
//				if err != nil {
//					t.Fatal(err)
//				}
//				fmt.Println(string(out))
//			}
//		}
//
//		key := zone + "," + insType + "," + insChargeType
//		fmt.Println(key)
//		if priceItem, ok := standardPricings[key]; ok {
//			out, err := json.MarshalIndent(priceItem.Price, "", "  ")
//			if err != nil {
//				t.Fatal(err)
//			}
//			fmt.Println(string(out))
//		}
//
//	}
//
//	prices, err := cvmClient.GetCVMInstancesInquiryPrice(instances)
//	if err != nil {
//		t.Fatal(err)
//	}
//	for _, price := range prices {
//		out, err := json.MarshalIndent(price, "", "  ")
//		if err != nil {
//			t.Fatal(err)
//		}
//		fmt.Println(string(out))
//	}
//}
