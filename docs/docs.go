// Package docs GENERATED BY SWAG; DO NOT EDIT
// This file was generated by swaggo/swag
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/getinfo/{path}": {
            "get": {
                "description": "Get info of URL",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "getinfo"
                ],
                "summary": "Get info of URL",
                "operationId": "get-info-url",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Info URL",
                        "name": "path",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/main.response"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/main.message"
                        }
                    }
                }
            }
        },
        "/shorted": {
            "post": {
                "description": "Create a shortened URL",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "shorten"
                ],
                "summary": "Shorten URL",
                "operationId": "shorten-url",
                "parameters": [
                    {
                        "description": "Original URL",
                        "name": "shorten",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/main.shortenBody"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/main.response"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/main.message"
                        }
                    }
                }
            }
        },
        "/{path}": {
            "get": {
                "description": "Redirect to original URL",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "redirect"
                ],
                "summary": "Redirect to original URL",
                "operationId": "redirect-url",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Shortened URL",
                        "name": "path",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {}
            }
        }
    },
    "definitions": {
        "main.message": {
            "type": "object",
            "properties": {
                "message": {
                    "type": "string"
                }
            }
        },
        "main.response": {
            "type": "object",
            "properties": {
                "message": {
                    "type": "string"
                },
                "status": {
                    "type": "string"
                },
                "url": {
                    "$ref": "#/definitions/urlshortenerpb.ShortenedURL"
                }
            }
        },
        "main.shortenBody": {
            "type": "object",
            "properties": {
                "original_url": {
                    "type": "string"
                },
                "shorted_url": {
                    "type": "string"
                }
            }
        },
        "urlshortenerpb.ShortenedURL": {
            "type": "object",
            "properties": {
                "clicks": {
                    "type": "integer"
                },
                "originalURL": {
                    "type": "string"
                },
                "shortenedURL": {
                    "type": "string"
                }
            }
        }
    },
    "securityDefinitions": {
        "BasicAuth": {
            "type": "basic"
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.4.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http", "https"},
	Title:            "URL Shortener API",
	Description:      "This is a server for URL Shortener API.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
