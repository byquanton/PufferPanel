/*
 Copyright 2022 (c) PufferPanel
  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at
  	http://www.apache.org/licenses/LICENSE-2.0
  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package pufferpanel

type Stat struct {
	Cpu    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
} //@name Stat

type Log struct {
	Epoch int64  `json:"epoch"`
	Logs  string `json:"logs"`
} //@name Log

type Running struct {
	Running bool `json:"running"`
} //@name Running

type Variables struct {
	Variables map[string]Variable `json:"data"`
} //@name Variables

type Tasks struct {
	Tasks map[string]ServerTask `json:"tasks"`
} //@name Tasks

type ServerTask struct {
	Running
	Task
} //@name Task

type OAuthTokenRequest struct {
	GrantType    string `form:"grant_type"`
	ClientId     string `form:"client_id"`
	ClientSecret string `form:"client_secret"`
	Username     string `form:"username"`
	Password     string `form:"password"`
} //@name OAuth2TokenRequest

type OAuthTokenInfoResponse struct {
	Active bool   `json:"active"`
	Scope  string `json:"scope,omitempty"`
} //@name OAuth2TokenInfoResponse

type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
} //@name OAuth2TokenResponse

type OAuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
} //@name OAuth2ErrorResponse

type OAuthServerResponse struct {
	OAuthTokenResponse
	OAuthErrorResponse
}

type ErrorResponse struct {
	Error *Error `json:"error"`
} //@name ErrorResponse

type Metadata struct {
	Paging *Paging `json:"paging"`
} //@name Metadata

type Paging struct {
	Page    uint  `json:"page"`
	Size    uint  `json:"pageSize"`
	MaxSize uint  `json:"maxSize"`
	Total   int64 `json:"total"`
} //@name Paging

type Features struct {
	Features     []string `json:"features"`
	Environments []string `json:"environments"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
} //@name Features

type EditableConfig struct {
	Themes              ThemeConfig    `json:"themes"`
	Branding            BrandingConfig `json:"branding"`
	RegistrationEnabled bool           `json:"registrationEnabled"`
} //@name EditableConfigSettings

type ThemeConfig struct {
	Active    string   `json:"active"`
	Settings  string   `json:"settings"`
	Available []string `json:"available"`
} //@name ThemeConfig

type BrandingConfig struct {
	Name string `json:"name"`
} //@name BrandingConfig
