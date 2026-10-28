include ./NickelHook/NickelHook.mk

override LIBRARY  := libkscribbler.so
override SOURCES  += $(wildcard src/*.cc) qrcodegen/c/qrcodegen.c
override CFLAGS   += -Wall -Wextra -Werror -Wno-missing-field-initializers -fvisibility=hidden -Iqrcodegen/c
override CXXFLAGS += -Wall -Wextra -Werror -Wno-missing-field-initializers -fvisibility=hidden -fvisibility-inlines-hidden -Iqrcodegen/c
override PKGCONF  += Qt5Widgets
override MOCS     += $(wildcard src/*.h)

include ./NickelHook/NickelHook.mk
