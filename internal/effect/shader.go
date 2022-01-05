package effect

var ShaderFrag = []byte(`package main

var (
	Color      float
	Brightness float
	Ghost      float
)

func convertRGB2HSV(rgb vec3) vec3 {
	// Hue calculation has 3 cases, depending on which RGB component is largest, and one of those cases involves a "mod"
	// operation. In order to avoid that "mod" we split the M==R case in two: one for G<B and one for B>G. The B>G case
	// will be calculated in the negative and fed through abs() in the hue calculation at the end.
	// See also: https://en.wikipedia.org/wiki/HSL_and_HSV#Hue_and_chroma
	var hueOffsets vec4 = vec4(0.0, -1.0/3.0, 2.0/3.0, -1.0)

	// temp1.xy = sort B & G (largest first)
	// temp1.z = the hue offset we'll use if it turns out that R is the largest component (M==R)
	// temp1.w = the hue offset we'll use if it turns out that R is not the largest component (M==G or M==B)
	var temp1 vec4
	if rgb.b > rgb.g {
		temp1 = vec4(rgb.bg, hueOffsets.wz)
	} else {
		temp1 = vec4(rgb.gb, hueOffsets.xy)
	}

	// temp2.x = the largest component of RGB ("M" / "Max")
	// temp2.yw = the smaller components of RGB, ordered for the hue calculation (not necessarily sorted by magnitude!)
	// temp2.z = the hue offset we'll use in the hue calculation
	var temp2 vec4
	if rgb.b > rgb.g {
		temp2 = vec4(rgb.r, temp1.yzx)
	} else {
		temp2 = vec4(temp1.xyw, rgb.r)
	}

	var m, C, V float
	// m = the smallest component of RGB ("min")
	m = min(temp2.y, temp2.w)

	// Chroma = M - m
	C = temp2.x - m

	// Value = M
	V = temp2.x

	var epsilon float = 1e-3

	return vec3(
		abs(temp2.z+(temp2.w-temp2.y)/(6.0*C+epsilon)), // Hue
		C/(temp2.x+epsilon),                            // Saturation
		V)                                              // Value
}

func convertHue2RGB(hue float) vec3 {
	var r float = abs(hue*6.0-3.0) - 1.0
	var g float = 2.0 - abs(hue*6.0-2.0)
	var b float = 2.0 - abs(hue*6.0-4.0)
	return clamp(vec3(r, g, b), 0.0, 1.0)
}

func convertHSV2RGB(hsv vec3) vec3 {
	var rgb vec3 = convertHue2RGB(hsv.x)
	var c float = hsv.z * hsv.y
	return rgb*c + hsv.z - c
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	var txtcolor vec4
	var source_size  vec2 = imageSrcTextureSize()
	var texel_size   vec2= 1.0 / source_size

	var p0  vec2= texCoord - (texel_size) / 2.0 + (texel_size / 512.0)
	var p1  vec2= texCoord + (texel_size) / 2.0 + (texel_size / 512.0)
  


	var c0 vec4= imageSrc0At( p0)
	var c1 vec4= imageSrc0At( vec2(p1.x, p0.y))
	var c2 vec4= imageSrc0At( vec2(p0.x, p1.y))
	var c3 vec4= imageSrc0At( p1)

  
	var rate  vec2= fract(p0 * source_size)
	txtcolor = mix(mix(c0, c1, rate.x), mix(c2, c3, rate.x), rate.y)
	if txtcolor.a == 0.0 {
		return vec4(0)
	}

	if Color > 0.0 {
		var hsv vec3 = convertRGB2HSV(txtcolor.xyz)
		const minLightness float = 0.11 / 2.0
		const minSaturation float = 0.09
		if hsv.z < minLightness {
			hsv = vec3(0.0, 1.0, minLightness)
		} else if hsv.y < minSaturation {
			hsv = vec3(0.0, minSaturation, hsv.z)
		}
		hsv.x = mod(hsv.x+Color/200.0, 1.0)
		if hsv.x < 0.0 {
			hsv.x += 1.0
		}
		var rgb vec3 = convertHSV2RGB(hsv)
		txtcolor = vec4(rgb, txtcolor.a)
	}

	if Brightness > 0.0 {
		var rgb vec3 =  clamp(txtcolor.rgb + vec3(min(Brightness/100.0, 1.0)), vec3(0), vec3(1))
		txtcolor = vec4(rgb,txtcolor.a)
	}

	//0 ~ 100
	if Ghost > 0.0{
		//1 - (Math.max(0, Math.min(x, 100)) / 100)
		txtcolor *= 1.0 - (max(0.0, min(Ghost, 100.0)) / 100.0)
	}

	return txtcolor
}
`)
