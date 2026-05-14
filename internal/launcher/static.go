package launcher

import (
	_ "embed"
	"html/template"
)

//go:embed static/js-yaml.min.js
var jsyamlSrc string

// InlineJSYaml is the js-yaml library wrapped to hide AMD/module globals
// so it correctly sets window.jsyaml in browser context.
var InlineJSYaml = template.JS("(function(){var _d=typeof define!=='undefined'?define:undefined;var _m=typeof module!=='undefined'?module:undefined;define=undefined;module=undefined;" + jsyamlSrc + ";define=_d;module=_m;})()")
