from pathlib import Path
import os
import datetime

def main(asdf):
    current_directory = Path.cwd()
    print(datetime.datetime.today())
    dirs_list = os.listdir()
    print(dirs_list)
    asdf = asdf + asdf
    return current_directory


def main2(asdf):
    for _ in asdf:
        print(_)


if __name__ == '__main__':    
    print(main(asdf="asdf"))
    
