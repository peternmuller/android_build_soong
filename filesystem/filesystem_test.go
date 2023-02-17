// Copyright 2021 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filesystem

import (
	"os"
	"testing"

	"android/soong/android"
	"android/soong/cc"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

var fixture = android.GroupFixturePreparers(
	android.PrepareForIntegrationTestWithAndroid,
	cc.PrepareForIntegrationTestWithCc,
	PrepareForTestWithFilesystemBuildComponents,
)

func TestFileSystemDeps(t *testing.T) {
	result := fixture.RunTestWithBp(t, `
		android_filesystem {
			name: "myfilesystem",
		}
	`)

	// produces "myfilesystem.img"
	result.ModuleForTests("myfilesystem", "android_common").Output("myfilesystem.img")
}

func TestFileSystemFillsLinkerConfigWithStubLibs(t *testing.T) {
	result := fixture.RunTestWithBp(t, `
		android_system_image {
			name: "myfilesystem",
			deps: [
				"libfoo",
				"libbar",
			],
			linker_config_src: "linker.config.json",
		}

		cc_library {
			name: "libfoo",
			stubs: {
				symbol_file: "libfoo.map.txt",
			},
		}

		cc_library {
			name: "libbar",
		}
	`)

	module := result.ModuleForTests("myfilesystem", "android_common")
	output := module.Output("system/etc/linker.config.pb")

	android.AssertStringDoesContain(t, "linker.config.pb should have libfoo",
		output.RuleParams.Command, "libfoo.so")
	android.AssertStringDoesNotContain(t, "linker.config.pb should not have libbar",
		output.RuleParams.Command, "libbar.so")
}

func registerComponent(ctx android.RegistrationContext) {
	ctx.RegisterModuleType("component", componentFactory)
}

func componentFactory() android.Module {
	m := &component{}
	m.AddProperties(&m.properties)
	android.InitAndroidArchModule(m, android.DeviceSupported, android.MultilibCommon)
	return m
}

type component struct {
	android.ModuleBase
	properties struct {
		Install_copy_in_data []string
	}
}

func (c *component) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	output := android.PathForModuleOut(ctx, c.Name())
	dir := android.PathForModuleInstall(ctx, "components")
	ctx.InstallFile(dir, c.Name(), output)

	dataDir := android.PathForModuleInPartitionInstall(ctx, "data", "components")
	for _, d := range c.properties.Install_copy_in_data {
		ctx.InstallFile(dataDir, d, output)
	}
}

func TestFileSystemGathersItemsOnlyInSystemPartition(t *testing.T) {
	f := android.GroupFixturePreparers(fixture, android.FixtureRegisterWithContext(registerComponent))
	result := f.RunTestWithBp(t, `
		android_system_image {
			name: "myfilesystem",
			multilib: {
				common: {
					deps: ["foo"],
				},
			},
			linker_config_src: "linker.config.json",
		}
		component {
			name: "foo",
			install_copy_in_data: ["bar"],
		}
	`)

	module := result.ModuleForTests("myfilesystem", "android_common").Module().(*systemImage)
	android.AssertDeepEquals(t, "entries should have foo only", []string{"components/foo"}, module.entries)
}

func TestAvbGenVbmetaImage(t *testing.T) {
	result := fixture.RunTestWithBp(t, `
		avb_gen_vbmeta_image {
			name: "input_hashdesc",
			src: "input.img",
			partition_name: "input_partition_name",
			salt: "2222",
		}`)
	cmd := result.ModuleForTests("input_hashdesc", "android_arm64_armv8-a").Rule("avbGenVbmetaImage").RuleParams.Command
	android.AssertStringDoesContain(t, "Can't find correct --partition_name argument",
		cmd, "--partition_name input_partition_name")
	android.AssertStringDoesContain(t, "Can't find --do_not_append_vbmeta_image",
		cmd, "--do_not_append_vbmeta_image")
	android.AssertStringDoesContain(t, "Can't find --output_vbmeta_image",
		cmd, "--output_vbmeta_image ")
	android.AssertStringDoesContain(t, "Can't find --salt argument",
		cmd, "--salt 2222")
}

func TestAvbAddHashFooter(t *testing.T) {
	result := fixture.RunTestWithBp(t, `
		avb_gen_vbmeta_image {
			name: "input_hashdesc",
			src: "input.img",
			partition_name: "input",
			salt: "2222",
		}

		avb_add_hash_footer {
			name: "myfooter",
			src: "input.img",
			filename: "output.img",
			partition_name: "mypartition",
			private_key: "mykey",
			salt: "1111",
			props: [
				{
					name: "prop1",
					value: "value1",
				},
				{
					name: "prop2",
					file: "value_file",
				},
			],
			include_descriptors_from_images: ["input_hashdesc"],
		}
	`)
	cmd := result.ModuleForTests("myfooter", "android_arm64_armv8-a").Rule("avbAddHashFooter").RuleParams.Command
	android.AssertStringDoesContain(t, "Can't find correct --partition_name argument",
		cmd, "--partition_name mypartition")
	android.AssertStringDoesContain(t, "Can't find correct --key argument",
		cmd, "--key mykey")
	android.AssertStringDoesContain(t, "Can't find --salt argument",
		cmd, "--salt 1111")
	android.AssertStringDoesContain(t, "Can't find --prop argument",
		cmd, "--prop 'prop1:value1'")
	android.AssertStringDoesContain(t, "Can't find --prop_from_file argument",
		cmd, "--prop_from_file 'prop2:value_file'")
	android.AssertStringDoesContain(t, "Can't find --include_descriptors_from_image",
		cmd, "--include_descriptors_from_image ")
}
