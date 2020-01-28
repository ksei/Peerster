Vue.component('modal', {
    template: '#modal-template'
  })

new Vue({
    el: '#app', 
    data: {
        ws: null, // websocket
        newMsg: '', // Holds new messages to be sent to the server
        chatContent: '', // A running list of chat messages displayed on the screen
        peerContent: '', // A running list of peers
        ipAddress: null, // ipAddressess of the peer
        searchKeywords : null,
        myIP: '',
        origins: ['Group'],
        searchMatches: [],
        metahashes: {},
        chatboxmsg : [],
        activeChat : '',
        userMessages : {},
        me: '',
        file: '',
        fileMetahash :'',
        showModal: false,
    },

    created: function() {
        var self = this;
        this.activeChat = 'Group';
        this.ws = new WebSocket('ws://' + window.location.host + '/ws');
        console.log('ws://' + window.location.host + '/ws')
        this.ws.addEventListener('message', function(e) {
            var msg = JSON.parse(e.data);
            console.log("Hey");
            console.log(msg);
            if (msg.type == 'Message'){
            if(!self.origins.includes(msg.origin) && msg.origin != self.me){
                self.origins.push(msg.origin)
                // self.originContent += '<div id="sidebar-user-box" @click="popupChat" class="'+(self.origins.length + 1 )+'" ><span id="slider-username">'+msg.origin+'</span></div>'
            }
            if(msg.message == ""){
                return;
            }
            self.userMessages['Group'] = self.userMessages['Group'] || []
            self.userMessages['Group'].push({"ip":msg.ipAddr,"text":msg.message,"origin":msg.origin})
            if (self.activeChat == 'Group'){    
                tmpMsg =  emojione.toImage(msg.message);
                tmpMsgChip = '<div class="chip" style="margin-left:5px">'+ '<img src="' + self.gravatarURL(msg.origin) + '">' + msg.origin + " (" + msg.ipAddr + ") "+ '</div>'
                if(msg.origin == self.me){
                    self.chatContent += '<div style="float:right;clear:both;display:table;">' + tmpMsg + tmpMsgChip + '</div><br/>'
                }
                else{
                    self.chatContent += '<div>' +  tmpMsgChip + tmpMsg +'</div><br/>'
                }
            
            var element = document.getElementById('chat-messages');
            element.scrollTop = element.scrollHeight; // Auto scroll to the bottom
            }
        } else if(msg.type == 'PrivateMessage') {
            // console.log("Got Private")
            self.userMessages[msg.origin] = self.userMessages[msg.origin] || []
            self.userMessages[msg.origin].push({"text":msg.message, "origin":msg.origin})
            if(self.activeChat == msg.origin){
                self.chatContent += '<div class="chip">' +
                '<img src="' + self.gravatarURL(msg.origin) + '">' // Avatar
                    + msg.origin 
                    + '</div>'
                    + emojione.toImage(msg.message) + '<br/>'; // Parse emojis
                
                var element = document.getElementById('chat-messages');
                element.scrollTop = element.scrollHeight; // Auto scroll to the bottom
            }

        }else if(msg.type == "SearchMatch") {
            self.searchMatches.push(msg.filename)
            self.metahashes[msg.filename] = msg.metahash
        }else if(msg.type == "PeerUpdate"){
            if(msg.ipAddr.includes("--me")){
                self.me = msg.me
                document.getElementById('myip').textContent = '  ' + msg.ipAddr.substr(0,msg.ipAddr.length -4);
                return
            }
            self.peerContent += '<div class="chip">'
            + '<img src="https://img.icons8.com/color/48/000000/online.png">' // Avatar
                    + msg.ipAddr
                + '</div>'
            var element1 = document.getElementById('peer-list');
            element1.scrollTop = element1.scrollHeight; // Auto scroll to the bottom
        }

        });

    },

    methods: {
        send: function () {
            if (this.newMsg != '') {
                if(this.activeChat=='Group'){
                    this.ws.send(
                        JSON.stringify({
                            type: 'Message',
                            message: $('<p>').html(this.newMsg).text() // Strip out html
                        }
                    ));
                } else {
                    this.ws.send(
                        JSON.stringify({
                            type: 'PrivateMessage',
                            message: $('<p>').html(this.newMsg).text(), // Strip out html
                            destination: this.activeChat,
                        }
                    ));
                    this.userMessages[this.activeChat] = this.userMessages[this.activeChat] || [];
                    this.userMessages[this.activeChat].push({"text":this.newMsg, "origin":this.me});
                    this.chatContent += '<div style="float:right;clear:both;display:table;">'+ emojione.toImage(this.newMsg)+ '<div class="chip" style="margin-left:5px">' +
                    '<img src="' + this.gravatarURL(this.me) + '">' // Avatar
                            + this.me 
                    + '</div></div>'  + '<br/>'
                }

                this.newMsg = ''; // Reset newMsg
            }
        },

        join: function () {
            if (!this.ipAddress) {
                Materialize.toast('You must enter an ipAddressess', 2000);
                return
            }
            this.ws.send(
                JSON.stringify({
                    type: 'PeerUpdate',
                    ipAddr: $('<p>').html(this.ipAddress).text() // Strip out html
                }
            ));
            this.ipAddress = '';
        },

        gravatarURL: function(email) {
            return 'http://www.gravatar.com/avatar/' + CryptoJS.MD5(email);
        },

        renderChatBox: function() {
            this.userMessages[this.activeChat] = this.userMessages[this.activeChat] || [];
            console.log(this.activeChat)
            console.log(this.userMessages[this.activeChat]);
            this.userMessages[this.activeChat].forEach(this.generateMessage);
            var element = document.getElementById('chat-messages');
            element.scrollTop = element.scrollHeight; // Auto scroll to the bottom
        },

        generateMessage: function(messageTuple){
            message = emojione.toImage(messageTuple.text)
            messageChip = '<div class="chip" style="margin-left:5px">' + '<img src="' + this.gravatarURL(messageTuple.origin) + '">' + messageTuple.origin ;
            if(this.activeChat == 'Group'){
                    messageChip +=" (" + messageTuple.ip + ") "}
            messageChip += '</div>'
            
            if(messageTuple.origin == this.me){
                this.chatContent += '<div style="float:right;clear:both;display:table;">' + message + messageChip + '</div> <br/>'
            }
            else{
            this.chatContent += '<div>' + messageChip + message + '</div><br/>'
            }
            
        },

        switchChat: function(event){
            console.log(this.userMessages);
            if(this.activeChat == event.target.innerText){
                return;
            }
            this.activeChat = event.target.innerText;
            this.chatContent = "";
            this.renderChatBox();
        },
        downloadFile: function(event){
            FileName = event.target.innerText;
            console.log(this.metahashes);
            FileHash = this.metahashes[FileName];
            console.log(FileName);
            if (FileHash != '') {
                this.ws.send(
                    JSON.stringify({
                        type: 'FileSharing',
                        filename: FileName,
                        metahash: FileHash,
                    }
                ));
        }
        },
        fileSelected: function(event){
            // console.log(event.target.files[0].name)
            tempFile = event.target.files[0].name
            if (this.tempFile != '') {
                this.ws.send(
                    JSON.stringify({
                        type: 'FileSharing',
                        filename: tempFile,
                    }
                ));
        }
        },

        requestFileDownload: function(){
            // console.log(this.file + ":" + this.fileMetahash)
            // console.log(this.activeChat)
            // console.log(event)
            if (this.file != '' && this.fileMetahash!= '') {
                    this.ws.send(
                        JSON.stringify({
                            type: 'FileSharing',
                            filename: this.file,
                            metahash: this.fileMetahash,
                            destination: this.activeChat,
                        }
                    ));
            }
            this.file='';
            this.fileMetahash = '';
            this.showModal = false;
            this.$emit('close')
        },
        searchFile: function(){
            if (!this.searchKeywords) {
                Materialize.toast('You must enter keywords split by comma', 2000);
                return
            }
            this.searchMatches = []
            this.ws.send(
                JSON.stringify({
                    type: 'SearchRequest',
                    keywords: $('<p>').html(this.searchKeywords).text(), // Strip out html
                }
            ));
            this.searchKeywords = '';
        },
        closeModal: function(){
            // console.log(this.file + ":" + this.fileMetahash)
            // console.log(this.activeChat)
            // console.log(event)
            this.file='';
            this.fileMetahash = '';
            this.showModal = false;
            this.$emit('close')
        },
    }
});