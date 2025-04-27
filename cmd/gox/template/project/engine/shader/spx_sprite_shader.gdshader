shader_type canvas_item;
render_mode blend_mix;

uniform float alpha_amount : hint_range(0.0, 1.0) = 0.0;
uniform float color_amount : hint_range(0.0, 1.0) = 0.0;
uniform float brightness_amount : hint_range(0.0, 1.0) = 0.0;
uniform float uv_amount : hint_range(0.001, 100.0) = 0.0;
uniform float pixleate_amount : hint_range(0.0, 100.0) = 0.0;
uniform float whirl_amount : hint_range(-20, 20.0) = 0.0;
uniform float fisheye_amount : hint_range(-1.0, 100.0) = 0.0;
uniform vec4 atlas_uv_rect2 = vec4(0.0, 0.0, 1.0, 1.0); // offset , size

vec3 rgb_to_hsl(vec3 c) {
    vec4 K = vec4(0.0, -1.0/3.0, 2.0/3.0, -1.0);
    vec4 p = mix(vec4(c.bg, K.wz), vec4(c.gb, K.xy), step(c.b, c.g));
    vec4 q = mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r));
    float d = q.x - min(q.w, q.y);
    float e = 1.0e-10;
    return vec3(abs(q.z + (q.w - q.y)/(6.0*d + e)), d/(q.x + e), q.x);
}

vec3 hsl_to_rgb(vec3 hsl) {
    vec4 K = vec4(1.0, 2.0/3.0, 1.0/3.0, 3.0);
    vec3 p = abs(fract(hsl.xxx + K.xyz) * 6.0 - K.www);
    return hsl.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), hsl.y);
}

void vertex() {

}


void fragment() {
	vec2 uv = UV;
	// pixleate effect
	if (pixleate_amount != 0.0) {
		float newAmount = pixleate_amount;
		vec2 pixelSize = vec2(newAmount);

		vec2 correction = TEXTURE_PIXEL_SIZE * vec2(float(pixelSize.x), float(pixelSize.y)) / vec2(2.0);
		vec2 texture_uv = floor(uv / TEXTURE_PIXEL_SIZE);
		vec2 offset = vec2(mod(texture_uv.x, pixelSize.x), mod(texture_uv.y, pixelSize.y));
		vec2 target = (texture_uv - offset) * TEXTURE_PIXEL_SIZE;
		uv = target + correction;
	}
	// remmap uv to [0, 1]
	uv = (uv - atlas_uv_rect2.xy)  / atlas_uv_rect2.zw;

	// whirl effect
	if (whirl_amount != 0.0) {
		float newAmount = -whirl_amount;

		vec2 kCenter = vec2(0.5);

		const float kRadius = 0.5;
		vec2 offset = uv - kCenter;
		float offsetMagnitude = length(offset);
		float whirlFactor = max(1.0 - (offsetMagnitude / kRadius), 0.0);
		float whirlActual = newAmount * whirlFactor * whirlFactor;
		float sinWhirl = sin(whirlActual);
		float cosWhirl = cos(whirlActual);
		mat2 rotationMatrix = mat2(vec2(cosWhirl, -sinWhirl),vec2(sinWhirl, cosWhirl));

		uv = rotationMatrix * offset + kCenter;
	}

	// fisheye effect
	if(fisheye_amount != 0.0){
		float newAmount = (fisheye_amount) + 1.0;

		vec2 kCenter = vec2(0.5);

		vec2 vec = (uv - kCenter) / kCenter;
		float vecLength = length(vec);
		float r = pow(min(vecLength, 1.0), newAmount) * max(1.0, vecLength);
		vec2 unit = vec / vecLength;

		uv = kCenter + r * unit * kCenter;
	}
	
	// uv effect
	if (uv_amount != 0.0) {
		uv *= uv_amount + 1.0;
		uv.x = mod(uv.x, 1.0);
		uv.y = mod(uv.y, 1.0);
	}

	// remmap uv to atlas
	uv = atlas_uv_rect2.xy + uv * atlas_uv_rect2.zw;

	vec3 color;
	float alpha = 1.0;
	// color effect
	vec4 sample_color = texture(TEXTURE, uv).rgba;
	color = sample_color.rgb;
	alpha = sample_color.a;

	if (color_amount != 0.0) {
		vec3 hsl = rgb_to_hsl(color);
		hsl.x = mod(hsl.x + color_amount, 1.0); // color effect
		color = hsl_to_rgb(hsl);
	}

	// brightness effect
	color += brightness_amount;

	// alpha effect 
	alpha *= clamp(1.0 - alpha_amount, 0.0, 1.0);

	COLOR.rgb = color;
	COLOR.a = alpha;
}
