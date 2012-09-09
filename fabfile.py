from fabric.api import local
import sys

def isRepoClean():
	res = local('git status -s', capture=True)
	return res == ""

def run():
	if not isRepoClean():
		print("Cannot proceed, git repository contains uncommited changes!")
		#print(local('git status -s', capture=True))
		sys.exit(1)
	else:
		print("repo is clean!")
