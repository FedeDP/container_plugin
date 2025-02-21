# SPDX-License-Identifier: Apache-2.0
#
# Copyright (C) 2024 The Falco Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
# the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
# specific language governing permissions and limitations under the License.
#

NAME := container

ifeq ($(OS),Windows_NT)
    detected_OS := Windows
else
    detected_OS := $(shell sh -c 'uname 2>/dev/null || echo Unknown')
endif

ifeq ($(detected_OS),Windows)
    OUTPUT := lib$(NAME).dll
else ifeq ($(detected_OS),Darwin)
	OUTPUT := lib$(NAME).dylib
else
	OUTPUT := lib$(NAME).so
endif

all: $(OUTPUT)

.PHONY: clean
clean:
	rm -rf build $(OUTPUT)
	make -C go-worker/ clean

# This Makefile requires CMake installed on the system
.PHONY: $(OUTPUT)
$(OUTPUT):
	cmake -B build -S . && make -C build/ container -j6 && cp build/$(OUTPUT) $(OUTPUT)

.PHONY: test
test: $(OUTPUT)
	make -C build/ test && build/test/test && make -C go-worker/ test

readme:
	@$(READMETOOL) -p ./$(OUTPUT) -f README.md
