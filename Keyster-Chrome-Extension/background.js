var websocket;
var connected = false;

chrome.runtime.onMessage.addListener(function(request, sender, sendResponse) {
    if (request.type == "bind") {
      createWebSocketConnection(request.params.socket);
      sendResponse();
    }
    else if (request.type == "checkConnection") {
        sendResponse(connected);
    }
    else if (request.type == "requestPassword"){
        requestPassword(request.params.account, request.params.user, request.params.master)
        sendResponse();
    }
    else if (request.type == "storePassword"){
        storePassword(request.params.account, request.params.user, request.params.master, request.params.newPassword)
        sendResponse();
    }
    else if (request.type == "deletePassword"){
        deletePassword(request.params.account, request.params.deleteUser, request.params.master)
        sendResponse();
    }
});

function requestPassword(refaccount, uname, master){
    websocket.send(JSON.stringify({type: "PasswordRequest", account: refaccount, username: uname, masterKey: master }));
}

function storePassword(refaccount, uname, master, newPass){
    websocket.send(JSON.stringify({type: "StorePasswordRequest", account: refaccount, username: uname, masterKey: master, password:newPass}));
}

function deletePassword(refaccount, deleteUsr, master){
    websocket.send(JSON.stringify({type: "PasswordDelete", account: refaccount, username: deleteUsr, masterKey: master }));
}

function createWebSocketConnection(address) {
    if('WebSocket' in window){
        chrome.storage.local.get("instance", function(data) {
            connect('ws://' + address + '/ws');
        });
    }
}

//Make a websocket connection with the server.
function connect(host) {
    websocket = new WebSocket(host);
    websocket.onmessage = function (event) {
        if (!connected){
            connected = true;
            chrome.runtime.sendMessage({ type: "connectionSuccessful" }, function (response) {
            });
        }
        var received_msg = JSON.parse(event.data);
        if (received_msg.type == "PasswordResult"){
            console.log(received_msg.password)
            chrome.runtime.sendMessage({ type: "passwordResult", params :{password: received_msg.password} }, function (response) {
            });
        }
        else if (received_msg.type == "PassowrdOpResult"){
            chrome.runtime.sendMessage({ type: "passwordOpResult", params :{result: received_msg.message} }, function (response) {
            });
        }
    };

    //If the websocket is closed but the session is still active, create new connection again
    websocket.onclose = function() {
        connected = false;
    };
}

//Close the websocket connection
function closeWebSocketConnection(username) {
    if (websocket != null || websocket != undefined) {
        websocket.close();
        websocket = undefined;
    }
}