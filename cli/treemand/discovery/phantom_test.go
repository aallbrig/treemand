package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// Lists that are not commands (issue #154), copied from corpus fixtures.
func TestParseHelp_noPhantomSubcommands(t *testing.T) {
	cases := map[string]string{
		"ruby features list": "Usage: ruby [switches] [--] [programfile] [arguments]\n" +
			"  -h              show this message, --help for more info\n" +
			"Features:\n" +
			"  gems            rubygems (only for debugging, default: enabled)\n" +
			"  did_you_mean    did_you_mean (default: enabled)\n",
		"ruby dump list": "Usage: ruby [switches] [--] [programfile] [arguments]\n" +
			"Dump List:\n" +
			"  insns           instruction sequences\n" +
			"  yydebug         yydebug of yacc parser generator\n",
		"ffmpeg library versions": "ffmpeg version 7.1 Copyright (c) 2000-2024 the FFmpeg developers\n" +
			"  built with gcc 14 (Ubuntu 14.2.0-4ubuntu2)\n" +
			"  libavutil      59.  8.100 / 59.  8.100\n" +
			"  libavcodec     61.  3.100 / 61.  3.100\n" +
			"Universal media converter\n" +
			"usage: ffmpeg [options] [[infile options] -i infile]...\n",
		"java usage continuation": "Usage: java [options] <mainclass> [args...]\n" +
			"           (to execute a class)\n" +
			"   or  java [options] -jar <jarfile> [args...]\n" +
			"           (to execute a jar file)\n",
		"tar format values": "Usage: tar [OPTION...] [FILE]...\n" +
			"GNU 'tar' saves many files together into a single tape or disk archive.\n" +
			"\n" +
			"  -H, --format=FORMAT        create archive of the given format\n" +
			"\n" +
			" FORMAT is one of the following:\n" +
			"\n" +
			"    gnu                      GNU tar 1.13.x format\n" +
			"    oldgnu                   GNU format as per tar <= 1.12\n",
	}
	for name, help := range cases {
		t.Run(name, func(t *testing.T) {
			if p := discovery.ParseHelpOutput(help); len(p.Subcommands) != 0 {
				t.Errorf("phantom subcommands: %v", p.Subcommands)
			}
		})
	}
}

// A free-form list introduced by a header that names commands still counts.
func TestParseHelp_commandListUnderAnUnknownHeaderStillCounts(t *testing.T) {
	help := "usage: git [-v | --version] <command> [<args>]\n\n" +
		"These are common Git commands used in various situations:\n\n" +
		"start a working area (see also: git help tutorial)\n" +
		"   clone     Clone a repository into a new directory\n" +
		"   init      Create an empty Git repository or reinitialize an existing one\n"
	p := discovery.ParseHelpOutput(help)
	if len(p.Subcommands) != 2 {
		t.Errorf("want clone and init, got %v", p.Subcommands)
	}
}
