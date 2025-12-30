import * as Lime from 'lime-js';
import WebSocketTransport from 'lime-transport-websocket'
import { v4 as uuidv4 } from 'uuid';

const msgerForm = get(".msger-inputarea");
const msgerInput = get(".msger-input");
const msgerChat = get(".msger-chat");
const msgerButton = get(".msger-send-btn");

// Icons made by Freepik from www.flaticon.com
const PERSON_IMG = "../images/person.png";
const BOT_IMG = "../images/bot.png";

let nickname = `guest`;
let client = await createClient();
let closing = false;

// Notify other users
await client.sendMessage({
    id: uuidv4(),
    from: nickname,
    type: 'application/x-chat-joined+json',
    content: {},
});

async function createClient() {
    setInputEnabled(false);
    try {
        let client = await connect();
        nickname = client.localNode.split("/")[0].split("@")[0];
        client.transport.onError = async function () {
            client = await connect();
        }
        client.transport.onClose = async function() {
            if (!closing) {
                client = await connect();
            }
        }
        client.onMessage = (message) => {
            switch (message.type) {
                case 'application/x-chat-nickname+json':
                    appendMessage("BOT", BOT_IMG, "left", `The user <strong>${message.content.old}</strong> has changed its nickname to <strong>${message.content.new}</strong>.`);
                    break;

                case 'application/x-chat-joined+json':
                    appendMessage("BOT", BOT_IMG, "left", `The user <strong>${message.from}</strong> has joined the room.`);
                    break;

                default: {
                    let senderName = message.from;
                    if (message.to) {
                        senderName += " (private)";
                    }
                    appendMessage(senderName, PERSON_IMG, "left", message.content);
                    break;
                }
            }
        }
        return client;
    } finally {
        setInputEnabled(true);
    }
}

msgerForm.addEventListener("submit", async event => {
    event.preventDefault();

    const msgText = msgerInput.value;
    if (!msgText) return;

    appendMessage(nickname, PERSON_IMG, "right", msgText);
    msgerInput.value = "";

    if (await parseCommand(msgText)) {
        return;
    }

    await sendMessage(msgText, 'text/plain');
});

async function sendMessage(content, type, to = null) {
    await client.sendMessage({
        id: uuidv4(),
        from: nickname,
        to: to,
        type: type ?? "text/plain",
        content: content,
    });
}



async function parseCommand(input) {
    let args = input.split(' ');

    switch (args[0]) {
        case '/name':
            if (args.length > 1) {
                await setNickname(args[1]);
                return true;
            }
            break;

        case '/to':
            if (args.length >= 2) {
                let to = args[1];
                let content = args.slice(2).join(' ');
                await sendMessage(content, 'text/plain', to);
                return true;
            }
            break;

        case '/friends':
            await getFriends();
            return true;

        case '/add':
            if (args.length > 1) {
                await addFriend(args[1]);
                return true;
            }
            break;

        case '/remove':
            if (args.length > 1) {
                await removeFriend(args[1]);
                return true;
            }
            break;
    }

    return false;
}

async function setNickname(newNickname) {
    let oldNickname = nickname;
    nickname = newNickname;

    closing = true;

    await client.sendFinishingSession();
    client = await createClient();

    closing = false;

    // Notify other users
    await client.sendMessage({
        id: uuidv4(),
        from: nickname,
        type: 'application/x-chat-nickname+json',
        content: {
            old: oldNickname,
            new: nickname,
        },
    });

    appendMessage("BOT", BOT_IMG, "left", `OK! Your name now is <strong>${nickname}</strong>.`);
}

function appendMessage(name, img, side, text) {
    //   Simple solution for small apps
    const msgHTML = `
    <div class="msg ${side}-msg">
      <div class="msg-img" style="background-image: url(${img})"></div>

      <div class="msg-bubble">
        <div class="msg-info">
          <div class="msg-info-name">${name}</div>
          <div class="msg-info-time">${formatDate(new Date())}</div>
        </div>

        <div class="msg-text">${text}</div>
      </div>
    </div>
  `;

    msgerChat.insertAdjacentHTML("beforeend", msgHTML);
    msgerChat.scrollTop += 500;
}

async function getFriends() {
    let response = await client.processCommand({
        id: uuidv4(),
        method: 'get',
        uri: '/friends'
    });

    if (response.status !== 'success') {
        appendMessage("BOT", BOT_IMG, "left", `Ops, an error occurred while retrieving your friends list: ${response.reason.description}`);
        return;
    }

    let responseMsg = 'Your friends are:<br>';
    for (let friend of response.resource.items) {
        responseMsg += `- ${friend.nickname}${friend.online? ' (online)': '' } <br>`;
    }

    appendMessage("BOT", BOT_IMG, "left", responseMsg);
}

async function addFriend(nickname) {
    let response = await client.processCommand({
        id: uuidv4(),
        method: 'set',
        uri: '/friends',
        type: 'application/x-chat-friend+json',
        resource: {
            nickname: nickname,
        }
    });

    if (response.status !== 'success') {
        appendMessage("BOT", BOT_IMG, "left", `Ops, an error occurred while adding this friends: ${response.reason.description}`);
        return;
    }

    appendMessage("BOT", BOT_IMG, "left", `The nickname '${nickname}' was added to your friends list.`);
}

async function removeFriend(nickname) {
    let response = await client.processCommand({
        id: uuidv4(),
        method: 'delete',
        uri: `/friends/${nickname}`
    });

    if (response.status !== 'success') {
        appendMessage("BOT", BOT_IMG, "left", `Ops, an error occurred while removing this friends: ${response.reason.description}`);
        return;
    }

    appendMessage("BOT", BOT_IMG, "left", `The nickname '${nickname}' was removed from your friends list.`);
}

// Utils
function get(selector, root = document) {
    return root.querySelector(selector);
}

function formatDate(date) {
    const h = "0" + date.getHours();
    const m = "0" + date.getMinutes();

    return `${h.slice(-2)}:${m.slice(-2)}`;
}

async function connect() {

    // Creates a new transport and connect to the server
    while (true) {
        try {
            let transport = new WebSocketTransport(null, false);
            await transport.open('ws://localhost:8080')
            let client = new Lime.ClientChannel(transport);

            await client.establishSession(
                'none',
                'none',
                nickname,
                new Lime.GuestAuthentication(),
                'chat');

            console.log("connected");

            return client;
        } catch (e) {
            console.error('Session establishment error', e);
            await new Promise(r => setTimeout(r, 1000));
        }
    }
}

function setInputEnabled(enabled) {
    if (enabled) {
        msgerButton.disabled = false;
        msgerInput.disabled = false;
        msgerInput.placeholder = "Enter your message...";

    } else {
        msgerButton.disabled = true;
        msgerInput.disabled = true;
        msgerInput.placeholder = "Connecting...";
    }
}
