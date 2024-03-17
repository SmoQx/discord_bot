import os
from pathlib import Path
import pathlib
import json


#client = discord.Client()
#
#
#async def server_name_create_jason() -> None:
#    print(client.guilds)
#    for guild in client.guilds:
#        with open(f"music_list_{guild}.json", mode="w") as create_json:
#            create_json.write()


def delete_json(name_to_delete: list) -> None:
    for name in os.listdir():
        file_name = pathlib.Path(f"{name}")
        if str(file_name) in name_to_delete:
            print(f"Removing {file_name}")
            try:
                print(f"{file_name} deleted successfully")
                os.remove(file_name)
            except OSError as e:
                print(f"Error while deleting {file_name}: {e}")


def read_music_list(file_name: pathlib.Path) -> dict:
    with open(file_name, mode="r") as read_json:
        music_dict = json.load(read_json)
    for key, value in dict(music_dict).items():
        print(key, value)
    print(music_dict)
    return music_dict


# TODO: 
# -- save all the song names
# -- delete all song with older timestamp
# -- create a json parser to save time of creation and tag of current song plating and the order of a queue which will change
# -- create a queue max len to not overload the file system
# -- create a json queue cleaner
#


if __name__ == "__main__":
#    server_name_create_jason()
#    delete_json(["new.txt", "new2.txt"])
    read_music_list(Path("file.json"))

