package csssafe

import (
	"regexp"
	"strings"
)

var (
	hexColorRe = regexp.MustCompile(`(?i)^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$`)
	rgbColorRe = regexp.MustCompile(`^rgba?\(\s*[0-9.,%\s]+\)$`)
	lengthRe   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:px|pt|mm|cm|in|em|rem|%)$`)
)

var namedColors = map[string]bool{
	"aliceblue": true, "antiquewhite": true, "aquamarine": true, "azure": true,
	"beige": true, "bisque": true, "blanchedalmond": true,
	"black": true, "white": true, "red": true, "green": true, "blue": true,
	"yellow": true, "orange": true, "purple": true, "gray": true, "grey": true,
	"silver": true, "maroon": true, "olive": true, "lime": true, "aqua": true,
	"teal": true, "navy": true, "fuchsia": true, "magenta": true, "cyan": true,
	"pink": true, "brown": true, "gold": true, "transparent": true,
	"darkred": true, "darkgreen": true, "darkblue": true, "lightgray": true,
	"lightgrey": true, "lightblue": true, "lightgreen": true, "lightyellow": true,
	"chocolate": true, "coral": true, "cornflowerblue": true, "cornsilk": true,
	"crimson": true, "darkcyan": true, "darkgoldenrod": true, "darkgray": true,
	"darkgrey": true, "darkkhaki": true, "darkmagenta": true, "darkolivegreen": true,
	"darkorange": true, "darkorchid": true, "darksalmon": true, "darkseagreen": true,
	"darkslateblue": true, "darkslategray": true, "darkslategrey": true,
	"darkturquoise": true, "darkviolet": true, "deeppink": true, "deepskyblue": true,
	"dimgray": true, "dimgrey": true, "dodgerblue": true, "firebrick": true,
	"floralwhite": true, "forestgreen": true, "gainsboro": true, "ghostwhite": true,
	"goldenrod": true, "greenyellow": true, "honeydew": true, "hotpink": true,
	"indianred": true, "indigo": true, "ivory": true, "khaki": true, "lavender": true,
	"lavenderblush": true, "lawngreen": true, "lemonchiffon": true, "lightcoral": true,
	"lightcyan": true, "lightgoldenrodyellow": true, "lightpink": true,
	"lightsalmon": true, "lightseagreen": true, "lightskyblue": true,
	"lightslategray": true, "lightslategrey": true, "lightsteelblue": true,
	"limegreen": true, "linen": true, "mediumaquamarine": true, "mediumblue": true,
	"mediumorchid": true, "mediumpurple": true, "mediumseagreen": true,
	"mediumslateblue": true, "mediumspringgreen": true, "mediumturquoise": true,
	"mediumvioletred": true, "midnightblue": true, "mintcream": true,
	"mistyrose": true, "moccasin": true, "navajowhite": true, "oldlace": true,
	"olivedrab": true, "orangered": true, "orchid": true, "palegoldenrod": true,
	"palegreen": true, "paleturquoise": true, "palevioletred": true,
	"papayawhip": true, "peachpuff": true, "peru": true, "plum": true,
	"powderblue": true, "rebeccapurple": true, "rosybrown": true, "royalblue": true,
	"saddlebrown": true, "salmon": true, "sandybrown": true, "seagreen": true,
	"seashell": true, "sienna": true, "skyblue": true, "slateblue": true,
	"slategray": true, "slategrey": true, "snow": true, "springgreen": true,
	"steelblue": true, "tan": true, "thistle": true, "tomato": true,
	"turquoise": true, "violet": true, "wheat": true, "whitesmoke": true,
	"yellowgreen": true,
}

// Color returns v only when it is a constrained CSS color value suitable for
// inline styles.
func Color(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	switch {
	case hexColorRe.MatchString(v):
		return v
	case rgbColorRe.MatchString(v):
		return v
	case namedColors[strings.ToLower(v)]:
		return v
	}
	return ""
}

// Length returns v only when it is a simple CSS length used by layout previews.
func Length(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.EqualFold(v, "auto") {
		return "auto"
	}
	if v == "0" || lengthRe.MatchString(v) {
		return v
	}
	return ""
}

// FontFamily strips CSS-breaking characters from a font-family value.
func FontFamily(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.Map(func(r rune) rune {
		switch r {
		case '"', '\'', '<', '>', ';', '\\':
			return -1
		default:
			return r
		}
	}, v)
	return strings.TrimSpace(v)
}

// TextAlign returns one of the allowed text-align values.
func TextAlign(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "left", "right", "center", "justify":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}
