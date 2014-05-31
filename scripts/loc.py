#!/usr/bin/env python

import os

# generates tuples (dir, files), where dir is relative to current dir
def gen_files(dirs, file_matcher=None, dir_matcher=None):
    seen_dirs = {}
    top_dir = os.getcwd()
    for d in dirs:
        full_dir = os.path.abspath(d)
        dirs_to_visit = [full_dir]
        while len(dirs_to_visit) > 0:
            curr_dir = dirs_to_visit[0]
            dirs_to_visit = dirs_to_visit[1:]
            if dir_matcher and not dir_matcher(curr_dir):
                continue
            if curr_dir in seen_dirs:
                continue
            dirs_and_files = os.listdir(curr_dir)
            files = []
            for e in dirs_and_files:
                path = os.path.join(curr_dir, e)
                if os.path.islink(path):
                    continue
                if os.path.isdir(path):
                    dirs_to_visit.append(path)
                    continue
                if os.path.isfile(path):
                    if file_matcher and not file_matcher(path):
                        continue
                    files.append(e)
            if len(files) > 0:
                relative_dir = curr_dir[len(top_dir) + 1:]
                if relative_dir == "":
                    relative_dir = "."
                yield (relative_dir, files)


def go_files_matcher(path):
    return path.endswith(".go")


def go_dirs_matcher(path):
    if path.endswith("Godeps"):
        return False
    return True


def go_files_gen():
    for res in gen_files(".", go_files_matcher, go_dirs_matcher):
        yield res


def python_files_matcher(path):
    return path.endswith(".py")


def python_files_gen():
    for res in gen_files(".", python_files_matcher):
        yield res


def loc_for_file(filePath):
    loc = 0
    with open(filePath, "r") as f:
        for line in f:
            loc += 1
    return loc


def print_lines(files_generator):
    total = 0
    for el in files_generator:
        d = el[0]
        total_dir_count = 0
        file_line_counts = []
        for f in el[1]:
            p = os.path.join(d, f)
            n = loc_for_file(p)
            total_dir_count += n
            file_line_counts.append([f, n])
        print("%s: %d" % (d, total_dir_count))
        total += total_dir_count
        for el in file_line_counts:
            print(" %-25s %d" % (el[0], el[1]))
        print("")
    print("Total: %d" % total)


if __name__ == "__main__":
    print_lines(go_files_gen())
    print("")
    print_lines(python_files_gen())
