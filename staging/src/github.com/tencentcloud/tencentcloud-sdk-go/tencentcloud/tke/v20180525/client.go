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
    "context"
    "errors"
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
    tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

const APIVersion = "2018-05-25"

type Client struct {
    common.Client
}

// Deprecated
func NewClientWithSecretId(secretId, secretKey, region string) (client *Client, err error) {
    cpf := profile.NewClientProfile()
    client = &Client{}
    client.Init(region).WithSecretId(secretId, secretKey).WithProfile(cpf)
    return
}

func NewClient(credential common.CredentialIface, region string, clientProfile *profile.ClientProfile) (client *Client, err error) {
    client = &Client{}
    client.Init(region).
        WithCredential(credential).
        WithProfile(clientProfile)
    return
}


func NewGetPodSpecificationRequest() (request *GetPodSpecificationRequest) {
    request = &GetPodSpecificationRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    request.Init().WithApiInfo("tke", APIVersion, "GetPodSpecification")
    
    
    return
}

func NewGetPodSpecificationResponse() (response *GetPodSpecificationResponse) {
    response = &GetPodSpecificationResponse{
        BaseResponse: &tchttp.BaseResponse{},
    }
    return
}

// GetPodSpecification
// 获取 Pod 规格  
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PARAM = "InternalError.Param"
//  INTERNALERROR_UNEXCEPTEDINTERNAL = "InternalError.UnexceptedInternal"
func (c *Client) GetPodSpecification(request *GetPodSpecificationRequest) (response *GetPodSpecificationResponse, err error) {
    return c.GetPodSpecificationWithContext(context.Background(), request)
}

// GetPodSpecification
// 获取 Pod 规格  
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PARAM = "InternalError.Param"
//  INTERNALERROR_UNEXCEPTEDINTERNAL = "InternalError.UnexceptedInternal"
func (c *Client) GetPodSpecificationWithContext(ctx context.Context, request *GetPodSpecificationRequest) (response *GetPodSpecificationResponse, err error) {
    if request == nil {
        request = NewGetPodSpecificationRequest()
    }
    
    if c.GetCredential() == nil {
        return nil, errors.New("GetPodSpecification require credential")
    }

    request.SetContext(ctx)
    
    response = NewGetPodSpecificationResponse()
    err = c.Send(request, response)
    return
}

func NewGetPriceRequest() (request *GetPriceRequest) {
    request = &GetPriceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    request.Init().WithApiInfo("tke", APIVersion, "GetPrice")
    
    
    return
}

func NewGetPriceResponse() (response *GetPriceResponse) {
    response = &GetPriceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    }
    return
}

// GetPrice
// EKS询价接口
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PARAM = "InternalError.Param"
//  INTERNALERROR_TRADECOMMON = "InternalError.TradeCommon"
//  INTERNALERROR_UNEXCEPTEDINTERNAL = "InternalError.UnexceptedInternal"
func (c *Client) GetPrice(request *GetPriceRequest) (response *GetPriceResponse, err error) {
    return c.GetPriceWithContext(context.Background(), request)
}

// GetPrice
// EKS询价接口
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PARAM = "InternalError.Param"
//  INTERNALERROR_TRADECOMMON = "InternalError.TradeCommon"
//  INTERNALERROR_UNEXCEPTEDINTERNAL = "InternalError.UnexceptedInternal"
func (c *Client) GetPriceWithContext(ctx context.Context, request *GetPriceRequest) (response *GetPriceResponse, err error) {
    if request == nil {
        request = NewGetPriceRequest()
    }
    
    if c.GetCredential() == nil {
        return nil, errors.New("GetPrice require credential")
    }

    request.SetContext(ctx)
    
    response = NewGetPriceResponse()
    err = c.Send(request, response)
    return
}
