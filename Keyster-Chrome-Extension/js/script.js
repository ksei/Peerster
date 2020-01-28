
$(document).ready(function () {
    var domain;
    var password = [];

    $(function () {
        localStorage.clear();

        var connected;
        chrome.runtime.sendMessage({ type: "checkConnection" }, function (response) {
            //here response will be the word you want
            if (response)
                $('#bindingPanel').hide()
            else
                $('#registerPanel').hide()
        });

        $('#resultPanel').hide();
        $('#createPanel').hide();
        $('#pleaseWaitPanel').hide()
        $('#alertMsg').hide();
        $('#userPanel').hide();
        $('#deletePanel').hide();

        chrome.tabs.query({ 'active': true, 'lastFocusedWindow': true }, function (tabs) {
            var url = tabs[0].url;

            if (url.indexOf("://") > -1) domain = url.split('/')[2];
            else domain = url.split('/')[0];

            domain = domain.split(':')[0];
            $('[name="site"]').val(domain);
        });
    });

    chrome.runtime.onMessage.addListener(function (request, sender, sendResponse) {
        if (request.type == "passwordResult") {
            $('#pleaseWaitPanel').hide();
            $('#userPanel').show();
            console.log(request.params.password)
            $('#copyPass').val(request.params.password);
            $('#managed').show();
            sendResponse();
        }
        else if (request.type == "passwordOpResult") {
            $('#pleaseWaitPanel').hide();
            if (request.params.result.indexOf("Successfully") >= 0) {
                $('#successMsg').text(request.params.result)
                $('#successResult').show();
                $('#errorResult').hide();
                $('#resultPanel').show();
            } else {
                $('#errorResult').show();
                $('#errorResultMsg').text(request.params.result)
                $('#successResult').hide();
                $('#resultPanel').show();
            }
        }
    });

    $('#bindButton').click(function () {
        chrome.tabs.query({ 'active': true, 'lastFocusedWindow': true }, function (tabs) {
            var url = tabs[0].url;

            if (url.indexOf("://") > -1) domain = url.split('/')[2];
            else domain = url.split('/')[0];

            // domain = domain.split(':')[0];
            chrome.runtime.sendMessage({
                type: "bind", params: {
                    socket: domain,
                }
            });

            chrome.runtime.sendMessage({ type: "checkConnection" }, function (response) {
                //here response will be the word you want
                if (response) {
                    $('#bindButton').hide()
                    $('#loginButton').show()
                }
            });
        });
    });


    $('#usr').click(function (e) {
        $('#errorStatus').text("")
    });


    $('#pwd').click(function (e) {
        $('#errorStatus').text("")
    });


    $('#reqUsr').click(function (e) {
        $('#createErrorStatus').text("")
    });

    $('#reqPwd').click(function (e) {
        $('#createErrorStatus').text("")
    });
    $('#newPasss').click(function (e) {
        $('#createErrorStatus').text("")
    });

    $('#switchToCreateButton').click(function () {
        $('#registerPanel').hide();
        $('#createPanel').show();
    });

    $('#createButton').click(function () {
        console.log($('#reqUsr').val())
        console.log($('#newPass').val())
        console.log($('#reqPwd').val())

        if ($('#reqUsr').val().length == 0 || $('#newPass').val().length == 0 || $('#reqPwd').val().length == 0) {
            $('#createErrorStatus').text("Please review the details entered...")
        }
        else {
            username = $('#reqUsr').val();
            masterKey = $('#reqPwd').val();
            newPass = $('#newPass').val()
            refaccount = $('[name="site"]').val();

            chrome.runtime.sendMessage({ type: "storePassword", params: { user: username, master: masterKey, account: refaccount, newPassword: newPass } }, function (response) {

            });

            $('#alertMsg').hide();
            $('#createPanel').hide();
            $('#pleaseWaitMessage').val("Please wait while your password is being stored...");
            $('#pleaseWaitPanel').show();
        }
    });

    $('#switchToDeleteButton').click(function () {
        if ($('#usr').val().length == 0 || $('#pwd').val().length == 0) {
            $('#errorStatus').text("Please review the details entered...")
        }
        else {
            $('#alertMsg').hide();
            $('#registerPanel').hide();
            $('#deletePanel').show();
        }
    });

    $('#deleteButton').click(function () {
        username = $('#usr').val();
        masterKey = $('#pwd').val();
        refaccount = $('[name="site"]').val();

        chrome.runtime.sendMessage({ type: "deletePassword", params: { deleteUser: username, master: masterKey, account: refaccount } }, function (response) {
        });
        $('#deletePanel').hide();
        $('#pleaseWaitMessage').val("Deleting your password...");
        $('#pleaseWaitPanel').show();
    });

    $('#cancelDeleteButton').click(function () {
        $('#registerPanel').show();
        $('#deletePanel').hide();
    });
    // :: function
    // :: Login Account
    // Description: Checks credentials and logs into password manager
    $('#loginButton').click(function () {
        if ($('#usr').val().length == 0 || $('#pwd').val().length == 0) {
            $('#errorStatus').text("Please review the details entered...")
        }
        else {
            username = $('#usr').val();
            masterKey = $('#pwd').val();
            refaccount = $('[name="site"]').val();

            chrome.runtime.sendMessage({ type: "requestPassword", params: { user: username, master: masterKey, account: refaccount } }, function (response) {

            });

            $('#alertMsg').hide();
            $('#registerPanel').hide();
            $('#pleaseWaitMessage').val("Please wait while your password is being retrieved...");
            $('#pleaseWaitPanel').show();
        }
    });

    // :: function
    // :: Slider Change
    // Description: update slider value on #pLen element
    $('input[name=plength]').change(function () {
        $('#pLen').text($('input[name=plength]').val());
    });

    // :: function
    // :: Copy to Clipboard
    // Description: Copies password generated by password gen to buffer
    $('#copyButtonGen').click(function () {
        copyToClipboard(document.getElementById("gpwd"));
    });

    $('#copyButtonUser').click(function () {
        copyToClipboard(document.getElementById("copyUser"));
    });

    $('#copyButtonPass').click(function () {
        copyToClipboard(document.getElementById("copyPass"));
    });

    // :: function
    // :: Password Generator
    // Description: Generates Password According to checked Checkboxes and Specified Length
    $('#generate').click(function () {
        var array = [];

        var alphabet = "abcdefghijklmnopqrstuvwxyz".split("");
        for (i = 0; i < alphabet.length; ++i) array.push(alphabet[i]);

        if ($('input[name=upper]').is(':checked')) {
            var alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
            for (i = 0; i < alphabet.length; ++i) array.push(alphabet[i]);
        }

        if ($('input[name=numeric]').is(':checked')) {
            var number = '0123456789'.split('');
            for (i = 0; i < number.length; ++i) array.push(number[i]);
        }

        if ($('input[name=spChar]').is(':checked')) {
            var special = '!@#$%^&*(){}][:;><,.]'.split('');
            for (i = 0; i < special.length; ++i) array.push(special[i]);
        }

        var pass = "";
        for (i = 0; i < parseInt($('input[name=plength]').val()); ++i) {
            pass += array[Math.floor(Math.random() * array.length)];
        }

        $('#gpwd').val(pass);
    });
    // :: function
    // :: Change Credentials
    // Description: Changes Credential for a website 
    $('#change').click(function () {
        $('#copyUser').attr('disabled', false);
        $('#copyPass').attr('disabled', false);

        $('#copyButtonUser').hide();
        $('#copyButtonPass').hide();

        $('#change').hide();
        $('#saveChange').show();
    });

    // :: function
    // :: Save Password
    // Description: Overwrites Password For a Particular Website using Encryption Techniques
    $('#saveChange').click(function () {
        if ($('#copyUser').val().length == 0 || $('#copyPass').val().length == 0) console.log('error:: username or master missing');
        else {
            localStorage.setItem(domain + 'id', $('#copyUser').val());

            var ekey = pseudoRandom();
            localStorage.setItem(domain + 'ekey', ekey);

            var ency = GibberishAES.enc($('#copyPass').val(), ekey);
            localStorage.setItem(domain + 'pass', ency);

            console.log('User: ' + $('#copyUser').val() + '\nEnc: ' + ency + '\n' + ekey + '\nDec: ' + GibberishAES.dec(ency, ekey));

            $('#copyUser').val(localStorage.getItem(domain + 'id'));
            $('#copyPass').val(localStorage.getItem(domain + 'pass'));

            $('#copyUser').attr('disabled', true);
            $('#copyPass').attr('disabled', true);

            $('#copyButtonUser').show();
            $('#copyButtonPass').show();

            $('#change').show();
            $('#saveChange').hide();
        }
    });
});

function copyToClipboard(elementId) {
    var aux = document.createElement("input");

    aux.setAttribute("value", $('#' + elementId.id).val());
    document.body.appendChild(aux);
    aux.select();
    document.execCommand("copy");

    document.body.removeChild(aux);
}

function pseudoRandom() {
    var array = [];

    var alphabet = "abcdefghijklmnopqrstuvwxyz".split("");
    for (i = 0; i < alphabet.length; ++i) array.push(alphabet[i]);

    alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
    for (i = 0; i < alphabet.length; ++i) array.push(alphabet[i]);

    var number = '0123456789'.split('');
    for (i = 0; i < number.length; ++i) array.push(number[i]);


    var special = '!@#$%^&*(){}][:;><,.]'.split('');
    for (i = 0; i < special.length; ++i) array.push(special[i]);


    var pass = "";
    for (i = 0; i < 16; ++i) {
        pass += array[Math.floor(Math.random() * array.length)];
    }

    return pass;
}