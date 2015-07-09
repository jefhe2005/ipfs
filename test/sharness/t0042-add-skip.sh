#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_add_skip_hidden() {

	test_expect_success "'ipfs add -r' with hidden file succeeds" '
		mkdir -p mountdir/planets/.asteroids &&
		echo "Hello Mars" >mountdir/planets/mars.txt &&
		echo "Hello Venus" >mountdir/planets/venus.txt &&
		echo "Hello Pluto" >mountdir/planets/.pluto.txt &&
		echo "Hello Charon" >mountdir/planets/.charon.txt &&
		echo "Hello Ceres" >mountdir/planets/.asteroids/ceres.txt &&
		echo "Hello Pallas" >mountdir/planets/.asteroids/pallas.txt &&
		ipfs add -r mountdir/planets >actual
	'

	test_expect_success "'ipfs add -r' did not include . files" '
		echo "added QmZy3khu7qf696i5HtkgL2NotsCZ8wzvNZJ1eUdA5n8KaV mountdir/planets/mars.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz mountdir/planets/venus.txt
added QmR8nD1Vzk5twWVC6oShTHvv7mMYkVh6dApCByBJyV2oj3 mountdir/planets" >expected &&
		test_cmp expected actual
	'

	test_expect_success "'ipfs add -r --hidden' succeeds" '
		ipfs add -r --hidden mountdir/planets >actual
	'

	test_expect_success "'ipfs add -r --hidden' did include . files" '
		echo "added QmcAREBcjgnUpKfyFmUGnfajA1NQS5ydqRp7WfqZ6JF8Dx mountdir/planets/.asteroids/ceres.txt
added QmZ5eaLybJ5GUZBNwy24AA9EEDTDpA4B8qXnuN3cGxu2uF mountdir/planets/.asteroids/pallas.txt
added Qmf6rbs5GF85anDuoxpSAdtuZPM9D2Yt3HngzjUVSQ7kDV mountdir/planets/.asteroids
added QmaowqjedBkUrMUXgzt9c2ZnAJncM9jpJtkFfgdFstGr5a mountdir/planets/.charon.txt
added QmU4zFD5eJtRBsWC63AvpozM9Atiadg9kPVTuTrnCYJiNF mountdir/planets/.pluto.txt
added QmZy3khu7qf696i5HtkgL2NotsCZ8wzvNZJ1eUdA5n8KaV mountdir/planets/mars.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz mountdir/planets/venus.txt
added QmetajtFdmzhWYodAsZoVZSiqpeJDAiaw2NwbM3xcWcpDj mountdir/planets" >expected &&
		test_cmp expected actual
	'

	test_expect_success "'remove planets dir' succeeds" '
		rm -rf mountdir/planets
	'

}

test_add_skip_ignore() {

	test_expect_success "'ipfs add -r' should succeed" '
		mkdir -p mountdir/planets &&
		echo "Hello Mars" >mountdir/planets/mars.txt &&
		echo "Hello Venus" >mountdir/planets/venus.txt &&
		echo "ignore*" >mountdir/planets/.ipfsignore &&
		echo "this file should be ignored" >mountdir/planets/ignore_me.txt &&
		ipfs add -r mountdir/planets >actual
  '

	test_expect_success "'ipfs add -r' respects .ipfsignore in active folder" '
		echo "added QmZy3khu7qf696i5HtkgL2NotsCZ8wzvNZJ1eUdA5n8KaV mountdir/planets/mars.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz mountdir/planets/venus.txt
added QmR8nD1Vzk5twWVC6oShTHvv7mMYkVh6dApCByBJyV2oj3 mountdir/planets" >expected &&
		test_cmp expected actual
	'

	test_expect_success "'ipfs add -r' respects... succeeded" '
		mkdir -p mountdir/planets/moons &&
		echo "mars*" >mountdir/.ipfsignore &&
		echo "ignore*" >mountdir/planets/.ipfsignore &&
		echo "europa*" >mountdir/planets/moons/.ipfsignore &&
		echo "Hello Mars" >mountdir/planets/mars.txt &&
		echo "Hello Venus" >mountdir/planets/venus.txt &&
		echo "Hello IO" >mountdir/planets/moons/io.txt &&
		echo "Hello Europa" >mountdir/planets/moons/europa.txt &&
		echo "this file should be ignored" >mountdir/planets/ignore_me.txt &&
		ipfs add -r mountdir/planets > recursive_ignore
	'

	test_expect_success "'ipfs add -r' respects root .ipfsignore files" '
		echo "added Qmc5bCQDyABcQWLDU7Q25yr3cJ3BLAbTRej4U8g4RqKbeE mountdir/planets/moons/io.txt
added QmcLVptZSfjVKBSCND7pZotAZ9oxRZ25m33imH8YuGwv6z mountdir/planets/moons
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz mountdir/planets/venus.txt
added Qme85NByZftwvZRcJ2PaGgURTzjBd5hry1aqudEpMsYJE3 mountdir/planets" >expected &&
		test_cmp expected recursive_ignore
	'

	test_expect_success "'clean up' succeeds" '
		rm -rf mountdir/planets &&
		rm mountdir/.ipfsignore
	'

}

# should work offline
test_init_ipfs

test_add_skip_hidden

test_add_skip_ignore

# should work online
test_launch_ipfs_daemon

test_add_skip_hidden

test_add_skip_ignore

test_kill_ipfs_daemon

test_done
