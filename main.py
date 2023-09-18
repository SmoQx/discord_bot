from responses import response_message

if __name__ == '__main__':
    message = ["!play", "LINK"]
    message2 = "!play LINK"
    lista = [message, message2, message3 := "!p", message4 := "!h"]
    for el in lista:
        print(response_message(el))
