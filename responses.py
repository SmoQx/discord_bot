def response_message(message: str or list):

    if type(message) == str:
        message = message.split(sep=" ")
        p_message = [lower_message.lower() for lower_message in message]
    elif type(message) == list:
        p_message = [lower_message.lower() for lower_message in message]
    else:
        p_message = "Error"

    if p_message[0] == '!h':
        response = "To play type !p \n" \
                   "For next song type !n \n" \
                   "To see whats in the Queue type !wq \n" \
                   "To add to Queue type !q \n" \
                   "To pause type !p \n"
        return response

    if p_message[0] == '!p':
        return "Now playing! "

    if p_message[0] == '!n':
        return "Next song is: "

    if p_message[0] == '!wq':
        pass

    if p_message[0] == '!q':
        pass
