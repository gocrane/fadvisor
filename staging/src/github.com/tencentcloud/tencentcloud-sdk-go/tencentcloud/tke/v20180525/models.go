// Copyright (c) 2017-2018 THL A29 Limited, a Tencent company. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v20180525

import (
    "encoding/json"
    tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
    tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
)

type GetPodSpecificationRequest struct {
	*tchttp.BaseRequest

	// 资源类型。CPU、V100、T4、1/4*V100等
	Type *string `json:"Type,omitempty" name:"Type"`

	// 资源需求描述
	ResourceRequirements []*string `json:"ResourceRequirements,omitempty" name:"ResourceRequirements"`
}

func (r *GetPodSpecificationRequest) ToJsonString() string {
    b, _ := json.Marshal(r)
    return string(b)
}

// FromJsonString It is highly **NOT** recommended to use this function
// because it has no param check, nor strict type check
func (r *GetPodSpecificationRequest) FromJsonString(s string) error {
	f := make(map[string]interface{})
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return err
	}
	delete(f, "Type")
	delete(f, "ResourceRequirements")
	if len(f) > 0 {
		return tcerr.NewTencentCloudSDKError("ClientError.BuildRequestError", "GetPodSpecificationRequest has unknown keys!", "")
	}
	return json.Unmarshal([]byte(s), &r)
}

type GetPodSpecificationResponse struct {
	*tchttp.BaseResponse
	Response *struct {

		// CPU核数
		Cpu *string `json:"Cpu,omitempty" name:"Cpu"`

		// 内存
		Memory *string `json:"Memory,omitempty" name:"Memory"`

		// GPU 卡数
		Gpu *int64 `json:"Gpu,omitempty" name:"Gpu"`

		// 唯一请求 ID，每次请求都会返回。定位问题时需要提供该次请求的 RequestId。
		RequestId *string `json:"RequestId,omitempty" name:"RequestId"`
	} `json:"Response"`
}

func (r *GetPodSpecificationResponse) ToJsonString() string {
    b, _ := json.Marshal(r)
    return string(b)
}

// FromJsonString It is highly **NOT** recommended to use this function
// because it has no param check, nor strict type check
func (r *GetPodSpecificationResponse) FromJsonString(s string) error {
	return json.Unmarshal([]byte(s), &r)
}

type GetPriceRequest struct {
	*tchttp.BaseRequest

	// pod cpu规格
	Cpu *float64 `json:"Cpu,omitempty" name:"Cpu"`

	// pod 内存规格
	Mem *float64 `json:"Mem,omitempty" name:"Mem"`

	// 使用时长，单位：秒
	TimeSpan *uint64 `json:"TimeSpan,omitempty" name:"TimeSpan"`

	// 副本数
	GoodsNum *uint64 `json:"GoodsNum,omitempty" name:"GoodsNum"`

	// 可用区
	Zone *string `json:"Zone,omitempty" name:"Zone"`

	// pod 资源类型，v100，t4，amd， 默认为：intel
	Type *string `json:"Type,omitempty" name:"Type"`

	// gpu 卡数
	Gpu *float64 `json:"Gpu,omitempty" name:"Gpu"`

	// 竞价实例："spot"
	PodType *string `json:"PodType,omitempty" name:"PodType"`
}

func (r *GetPriceRequest) ToJsonString() string {
    b, _ := json.Marshal(r)
    return string(b)
}

// FromJsonString It is highly **NOT** recommended to use this function
// because it has no param check, nor strict type check
func (r *GetPriceRequest) FromJsonString(s string) error {
	f := make(map[string]interface{})
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return err
	}
	delete(f, "Cpu")
	delete(f, "Mem")
	delete(f, "TimeSpan")
	delete(f, "GoodsNum")
	delete(f, "Zone")
	delete(f, "Type")
	delete(f, "Gpu")
	delete(f, "PodType")
	if len(f) > 0 {
		return tcerr.NewTencentCloudSDKError("ClientError.BuildRequestError", "GetPriceRequest has unknown keys!", "")
	}
	return json.Unmarshal([]byte(s), &r)
}

type GetPriceResponse struct {
	*tchttp.BaseResponse
	Response *struct {

		// 询价结果，单位：分，打折后
		Cost *uint64 `json:"Cost,omitempty" name:"Cost"`

		// 询价结果，单位：分，折扣前
		TotalCost *uint64 `json:"TotalCost,omitempty" name:"TotalCost"`

		// 唯一请求 ID，每次请求都会返回。定位问题时需要提供该次请求的 RequestId。
		RequestId *string `json:"RequestId,omitempty" name:"RequestId"`
	} `json:"Response"`
}

func (r *GetPriceResponse) ToJsonString() string {
    b, _ := json.Marshal(r)
    return string(b)
}

// FromJsonString It is highly **NOT** recommended to use this function
// because it has no param check, nor strict type check
func (r *GetPriceResponse) FromJsonString(s string) error {
	return json.Unmarshal([]byte(s), &r)
}
