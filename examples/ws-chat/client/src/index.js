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
        client.onMessage = (message) => {
            switch (message.type) {
                case 'application/x-chat-nickname+json':
                    appendMessage("BOT", BOT_IMG, "left", `The user <strong>${message.content.old}</strong> has changed its nickname to <strong>${message.content.new}</strong>.`);
                    break;

                case 'application/x-chat-joined+json':
                    appendMessage("BOT", BOT_IMG, "left", `The user <strong>${message.from}</strong> has joined the room.`);
                    break;

                default:
                    name = message.from;
                    if (message.to) {
                        name += " (private)";
                    }
                    appendMessage(name, PERSON_IMG, "left", message.content);
                    break;
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

    if (await parseCommand(msgText)) {
        msgerInput.value = "";
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

    appendMessage(nickname, PERSON_IMG, "right", content);
    msgerInput.value = "";
}

async function parseCommand(input) {
    if (input.startsWith("/name ")) {
        let arg = input.split(" ")[1];
        if (arg) {
            await setNickname(arg);
            return true;
        }
    }

    if (input.startsWith("/to")) {
        let args = input.split(" ");
        if (args.length >= 2) {
            let to = args[1];
            let content = args.slice(2).join(" ");
            await sendMessage(content, "text/plain", to);
            return true;
        }
    }

    return false;
}

async function setNickname(newNickname) {
    let oldNickname = nickname;
    nickname = newNickname;

    await client.sendFinishingSession();
    client = await createClient();

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


