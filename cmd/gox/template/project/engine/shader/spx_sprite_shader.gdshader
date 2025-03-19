shader_type canvas_item;
render_mode blend_mix;

uniform float alpha_amount : hint_range(0.0, 100.0) = 0;
uniform float color_amount : hint_range(0.0, 1000.0) = 0.0;
uniform float brightness_amount : hint_range(0.0, 100.0) = 0.0;
uniform float uv_amount : hint_range(0.0, 100.0) = 0.0;
uniform float mosaic_amount : hint_range(0.0, 100.0) = 0.0;
uniform float whirl_amount : hint_range(-1000, 1000.0) = 0.0;
uniform float fisheye_amount : hint_range(0, 200.0) = 0.0;

void vertex() {

}

void fragment() {
	vec3 color;
	vec2 uv = UV;

	// mosaic effect
	if (mosaic_amount != 0.0) {
		float newAmount = mosaic_amount / 3.338;
		vec2 pixelSize = vec2(newAmount);

		vec2 correction = TEXTURE_PIXEL_SIZE * vec2(float(pixelSize.x), float(pixelSize.y)) / vec2(2.0);
		vec2 texture_uv = floor(uv / TEXTURE_PIXEL_SIZE);
		vec2 offset = vec2(mod(texture_uv.x, pixelSize.x), mod(texture_uv.y, pixelSize.y));
		vec2 target = (texture_uv - offset) * TEXTURE_PIXEL_SIZE;
		//color = textureLod(TEXTURE, target + correction, 0.0).xyz;
		uv = target + correction;
	}

	// whirl effect
	if (whirl_amount != 0.0) {
		float newAmount = -whirl_amount / 50.0;

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
		float newAmount = (fisheye_amount / 100.0) + 1.0;

		vec2 kCenter = vec2(0.5);

		vec2 vec = (uv - kCenter) / kCenter;
		float vecLength = length(vec);
		float r = pow(min(vecLength, 1.0), newAmount) * max(1.0, vecLength);
		vec2 unit = vec / vecLength;

		uv = kCenter + r * unit * kCenter;
	}
	
	// uv effect
	if (uv_amount != 0.0) {
		float newAmount = trunc(((abs(uv_amount) + 5.0) / 10.0) + 1.0);
		uv *= vec2(newAmount);
	}

	// color effect
	vec3 color_temp;
	{
		vec3 c = texture(TEXTURE, uv).xyz;
		vec4 K = vec4(0.0, -1.0 / 3.0, 2.0 / 3.0, -1.0);
		vec4 p = mix(vec4(c.bg, K.wz), vec4(c.gb, K.xy), step(c.b, c.g));
		vec4 q = mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r));
		float d = q.x - min(q.w, q.y);
		float e = 1.0e-10;
		color_temp = vec3(abs(q.z + (q.w - q.y) / (6.0 * d + e)), d / (q.x + e), q.x);
	}

	{
		vec3 c = vec3(color_temp.x + color_amount / 200.00000, color_temp.y, color_temp.z);
		vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);
		vec3 p = abs(fract(c.xxx + K.xyz) * 6.0 - K.www);
		color = c.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), c.y);
	}
	// brightness effect
	color += brightness_amount / 100.0;
	
	COLOR.rgb = color;
	COLOR.a *= 1.0 - alpha_amount / 100.0;
}
