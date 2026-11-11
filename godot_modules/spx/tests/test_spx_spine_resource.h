/**************************************************************************/
/*  test_spx_spine_resource.h                                                  */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#ifndef TEST_SPX_SPINE_RESOURCE_H
#define TEST_SPX_SPINE_RESOURCE_H

#include "../spx_build_config.gen.h"
#ifdef SPX_SPINE_ENABLED

#include "../thirdparty/spine_godot/SpineSkeletonFileResource.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"

#include <initializer_list>

namespace TestSpxSpineResource {

static Vector<uint8_t> header_bytes(std::initializer_list<uint8_t> p_tail) {
	Vector<uint8_t> bytes;
	bytes.resize(8);
	bytes.fill(0);
	for (uint8_t byte : p_tail) {
		bytes.push_back(byte);
	}
	return bytes;
}

static Vector<uint8_t> version_header(const String &p_version) {
	Vector<uint8_t> bytes = header_bytes({});
	const CharString version = p_version.utf8();
	bytes.push_back(version.length() + 1);
	for (int index = 0; index < version.length(); ++index) {
		bytes.push_back(version[index]);
	}
	return bytes;
}

static String write_binary(const TestSpx::TempDirectory &p_files, const Vector<uint8_t> &p_bytes) {
	const String path = p_files.file("skeleton.skel");
	Ref<FileAccess> output = FileAccess::open(path, FileAccess::WRITE);
	REQUIRE(output.is_valid());
	if (output.is_valid() && !p_bytes.is_empty()) {
		output->store_buffer(p_bytes.ptr(), p_bytes.size());
	}
	return path;
}

TEST_CASE("[SPX] Spine skeleton header rejects truncated and overflowing lengths") {
	TestSpx::TempDirectory files("spx_spine_header");
	Ref<SpineSkeletonFileResource> resource;
	resource.instantiate();
	const Vector<uint8_t> valid = version_header("4.2.99");
	for (int length = 0; length < valid.size(); ++length) {
		Vector<uint8_t> truncated = valid;
		truncated.resize(length);
		CHECK(resource->load_from_file(write_binary(files, truncated)) == ERR_INVALID_DATA);
		CHECK_FALSE(resource->is_binary());
	}
	for (const Vector<uint8_t> &invalid : {
			header_bytes({ 0 }), // Null version.
			header_bytes({ 1 }), // Empty version.
			header_bytes({ 0x80 }), // Incomplete varint.
			header_bytes({ 0x80, 0x80, 0x80, 0x80, 0x80, 0 }), // Six-byte varint.
			header_bytes({ 0xFF, 0xFF, 0xFF, 0xFF, 0x10 }), // uint32 overflow.
			header_bytes({ 0xFF, 0xFF, 0xFF, 0xFF, 0x0F }), // Length exceeds file.
			header_bytes({ 6, '4', '.', '2', 0, '1' }), // Embedded NUL.
		}) {
		CHECK(resource->load_from_file(write_binary(files, invalid)) == ERR_INVALID_DATA);
		CHECK_FALSE(resource->is_binary());
	}
}

TEST_CASE("[SPX] Spine skeleton version matches the runtime major and minor") {
	TestSpx::TempDirectory files("spx_spine_version");
	Ref<SpineSkeletonFileResource> resource;
	resource.instantiate();
	for (const String &version : { String("4.2"), String("4.2.99") }) {
		CHECK(resource->load_from_file(write_binary(files, version_header(version))) == OK);
		CHECK(resource->is_binary());
	}
	const Vector<uint8_t> previous = resource->get_binary();
	for (const String &version : { String(""), String("4.1"), String("4.20"), String("14.2"), String("4.2."), String("5.0") }) {
		CHECK(resource->load_from_file(write_binary(files, version_header(version))) == ERR_INVALID_DATA);
		CHECK(resource->get_binary() == previous);
	}
}

TEST_CASE("[SPX] Spine skeleton JSON loads without registering all JSON files") {
	TestSpx::TempDirectory files("spx_spine_json");
	Ref<SpineSkeletonFileResource> resource;
	resource.instantiate();
	REQUIRE(resource->load_from_file(write_binary(files, version_header("4.2.99"))) == OK);
	const String valid = "{\"skeleton\":{\"spine\":\"4.2.99\"}}";
	for (const String &extension : { String("json"), String("spjson"), String("spine-json") }) {
		const String name = "skeleton." + extension;
		files.write(name, valid);
		CHECK(resource->load_from_file(files.file(name)) == OK);
		CHECK_FALSE(resource->is_binary());
		CHECK(resource->get_binary().is_empty());
		CHECK(resource->get_json() == valid);
	}
	for (const String &invalid : { String("{}"), String("{\"skeleton\":{}}"), String("{\"skeleton\":{\"spine\":\"4.20\"}}") }) {
		files.write("skeleton.json", invalid);
		CHECK(resource->load_from_file(files.file("skeleton.json")) == ERR_INVALID_DATA);
		CHECK(resource->get_json() == valid);
	}
	CHECK(resource->load_from_file(write_binary(files, version_header("4.2"))) == OK);
	CHECK(resource->is_binary());
	CHECK(resource->get_json().is_empty());

	Ref<SpineSkeletonFileResourceFormatLoader> loader;
	loader.instantiate();
	List<String> extensions;
	loader->get_recognized_extensions(&extensions);
	CHECK_FALSE(extensions.find("json") != nullptr);
}

TEST_CASE("[SPX] Spine skeleton format loader propagates invalid header errors") {
	TestSpx::TempDirectory files("spx_spine_loader_error");
	Ref<SpineSkeletonFileResourceFormatLoader> loader;
	loader.instantiate();
	const String path = write_binary(files, header_bytes({ 0x80 }));
	Error error = OK;
	Ref<Resource> result = loader->load(path, "", &error, false, nullptr, ResourceFormatLoader::CACHE_MODE_IGNORE);
	CHECK(result.is_null());
	CHECK(error == ERR_INVALID_DATA);
}

} // namespace TestSpxSpineResource

#endif // SPX_SPINE_ENABLED
#endif // TEST_SPX_SPINE_RESOURCE_H
