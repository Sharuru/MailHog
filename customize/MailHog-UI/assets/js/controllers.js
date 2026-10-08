var mailtrapApp = angular.module('mailtrapApp', []);

function fitMailFrame(frame) {
  var doc;
  try {
    doc = frame.contentDocument || (frame.contentWindow && frame.contentWindow.document);
  } catch (e) {
    return;
  }
  if (!doc || !doc.documentElement) {
    return;
  }
  if (!doc.getElementById('mailtrap-frame-fit')) {
    var style = doc.createElement('style');
    style.id = 'mailtrap-frame-fit';
    style.textContent = 'html,body{height:auto!important;min-height:0!important;max-height:none!important;overflow:visible!important;}';
    (doc.head || doc.documentElement).appendChild(style);
  }
  if (!doc.documentElement.getAttribute('data-mailtrap-wheel')) {
    doc.documentElement.setAttribute('data-mailtrap-wheel', '1');
    doc.addEventListener('wheel', function(event) {
      var stage = frame.closest('.mailtrap-detail-body') || frame.closest('.mailtrap-stage');
      if (!stage) {
        return;
      }
      stage.scrollTop += event.deltaY;
      stage.scrollLeft += event.deltaX;
      event.preventDefault();
    }, {passive: false});
  }
  frame.style.height = '0px';
  var body = doc.body;
  var height = doc.documentElement.scrollHeight || 0;
  if (body) {
    height = Math.max(height, body.scrollHeight || 0, body.offsetHeight || 0);
  }
  frame.style.height = Math.max(height, 1) + 'px';
}

mailtrapApp.directive('targetBlank', function(){
  return {
    link : function(scope, element, attributes){
      element.on('load', function() {
        var frame = element[0];
        var a = element.contents().find('a');
        a.attr('target', '_blank');
        var refit = function() {
          fitMailFrame(frame);
        };
        refit();
        element.contents().find('img').on('load error', refit);
        window.setTimeout(refit, 50);
      });
    }
  };
});

function guid() {
  function s4() {
    return Math.floor((1 + Math.random()) * 0x10000)
               .toString(16)
               .substring(1);
  }
  return s4() + s4() + '-' + s4() + '-' + s4() + '-' +
         s4() + '-' + s4() + s4() + s4();
}

mailtrapApp.directive('ngKeyEnter', function () {
  return function (scope, element, attrs) {
    element.bind("keydown keypress", function (event) {
      if(event.which === 13) {
        scope.$apply(function (){
          scope.$eval(attrs.ngKeyEnter);
        });
        event.preventDefault();
      }
    });
  };
});

mailtrapApp.controller('MailCtrl', function ($scope, $http, $sce, $timeout) {
  $scope.host = apiHost;

  $scope.cache = {};
  $scope.previewAllHeaders = false;

  $scope.eventsPending = {};
  $scope.eventCount = 0;
  $scope.eventDone = 0;
  $scope.eventFailed = 0;

  $scope.hasEventSource = false;
  $scope.source = null;

  $scope.itemsPerPage = 50
  $scope.startIndex = 0

  if(typeof(Storage) !== "undefined") {
      $scope.itemsPerPage = parseInt(localStorage.getItem("itemsPerPage"), 10)
      if(!$scope.itemsPerPage) {
        $scope.itemsPerPage = 50;
        localStorage.setItem("itemsPerPage", 50)
      }
  }

  $scope.startMessages = 0
  $scope.countMessages = 0
  $scope.totalMessages = 0

  $scope.startSearchMessages = 0
  $scope.countSearchMessages = 0
  $scope.totalSearchMessages = 0

  $scope.jim = null

  $scope.smtpmech = "NONE"
  $scope.selectedOutgoingSMTP = ""
  $scope.saveSMTPServer = false;

  $(function() {
    $scope.openStream();
    if(typeof(Notification) !== "undefined") {
      Notification.requestPermission();
    }
  });

  $scope.getMoment = function(a) {
    return moment(a).locale('zh-cn');
  }

  $scope.backToInbox = function() {
    $scope.preview = null;
    $scope.searching = false;
  }
  $scope.listPosition = function() {
    var list = $scope.searching ? $scope.searchMessages : $scope.messages;
    var total = $scope.searching ? $scope.totalSearchMessages : $scope.totalMessages;
    var start = $scope.searching ? $scope.startSearchMessages : $scope.startMessages;
    if (!$scope.preview || !list || !list.length) {
      return "";
    }
    for (var i = 0; i < list.length; i++) {
      if (list[i].ID == $scope.preview.ID) {
        return (start + i + 1) + " / " + (total || list.length);
      }
    }
    return "";
  }

  $scope.backToInboxFirst = function() {
    $scope.preview = null;
    $scope.startIndex = 0;
    $scope.startMessages = 0;
    $scope.searching = false;
    $scope.refresh();
  }

  $scope.toggleStream = function() {
    $scope.source == null ? $scope.openStream() : $scope.closeStream();
  }
  $scope.openStream = function() {
    var host = $scope.host.replace(/^http/, 'ws') ||
               (location.protocol.replace(/^http/, 'ws') + '//' + location.hostname + (location.port ? ':' + location.port : '') + location.pathname);
    $scope.source = new WebSocket(host + 'api/v2/websocket');
    $scope.source.addEventListener('message', function(e) {
      $scope.$apply(function() {
        $scope.totalMessages++;
        if ($scope.startIndex > 0) {
          $scope.startIndex++;
          $scope.startMessages++;
          return
        }
        if ($scope.countMessages < $scope.itemsPerPage) {
          $scope.countMessages++;
        }
        var message = JSON.parse(e.data);
        $scope.messages.unshift(message);
        while($scope.messages.length > $scope.itemsPerPage) {
          $scope.messages.pop();
        }
        if(typeof(Notification) !== "undefined") {
          $scope.createNotification(message);
        }
      });
    }, false);
    $scope.source.addEventListener('open', function(e) {
      $scope.$apply(function() {
        $scope.hasEventSource = true;
      });
    }, false);
    $scope.source.addEventListener('error', function(e) {
      //if(e.readyState == EventSource.CLOSED) {
        $scope.$apply(function() {
          $scope.hasEventSource = false;
        });
      //}
    }, false);
  }
  $scope.closeStream = function() {
    $scope.source.close();
    $scope.source = null;
    $scope.hasEventSource = false;
  }

  $scope.createNotification = function(message) {
    var title = "新测试邮件到达：" + $scope.getSender(message);
    var options = {
      body: $scope.tryDecodeMime(message.Content.Headers["Subject"][0]),
      tag: "MailTrap",
      icon: "images/logo.png"
    };
    var notification = new Notification(title, options);
    notification.addEventListener('click', function(e) {
      $scope.selectMessage(message);
      window.focus();
      notification.close();
    });
  }

  $scope.tryDecodeMime = function(str) {
    return unescapeFromMime(str)
  }

  $scope.resizePreview = function() {
    $('iframe.mailtrap-html-frame').each(function() {
      fitMailFrame(this);
    });
  }

  $(window).on('resize.mailtrapPreview', function() {
    $scope.resizePreview();
  });

  $scope.getSender = function(message) {
    return $scope.tryDecodeMime($scope.getDisplayName(message.Content.Headers["From"][0]) ||
                                message.From.Mailbox + "@" + message.From.Domain);
  }

  $scope.addressEmail = function(value) {
    var decoded = $scope.tryDecodeMime(value || "").trim();
    var named = decoded.match(/<([^>]+)>/);
    return (named ? named[1] : decoded).trim().toLowerCase();
  }

  $scope.parseReceiver = function(message) {
    var identified = {};
    var wholeReceiver = (message.Raw && message.Raw.To) || [];
    message.pTo = [];
    message.pCc = [];
    message.pBcc = [];
    function take(header, bucket) {
      var raw = message.Content && message.Content.Headers && message.Content.Headers[header];
      if (!raw || !raw[0]) {
        return;
      }
      raw[0].split(',').forEach(function(value) {
        var decoded = $scope.tryDecodeMime(value.trim());
        if (!decoded) {
          return;
        }
        bucket.push(decoded);
        identified[$scope.addressEmail(decoded)] = true;
      });
    }
    take("To", message.pTo);
    take("Cc", message.pCc);
    message.pBcc = wholeReceiver.filter(function(receiver) {
      return receiver && !identified[$scope.addressEmail(receiver)];
    });
  }

  $scope.getDisplayName = function(value) {
    if(!value) { return ""; }

    res = value.match(/(.*)\<(.*)\>/);

    if(res) {
      if(res[1].trim().length > 0) {
        return res[1].trim();
      }
      return res[2];
    }
    return value
  }

  $scope.startEvent = function(name, args, glyphicon) {
    var eID = guid();
    //console.log("Starting event '" + name + "' with id '" + eID + "'")
    var e = {
      id: eID,
      name: name,
      started: new Date(),
      complete: false,
      failed: false,
      args: args,
      glyphicon: glyphicon,
      getClass: function() {
        // FIXME bit nasty
        if(this.failed) {
          return "bg-danger"
        }
        if(this.complete) {
          return "bg-success"
        }
        return "bg-warning"; // pending
      },
      done: function() {
        //delete $scope.eventsPending[eID]
        var e = this;
        e.complete = true;
        $scope.eventDone++;
        if(this.failed) {
          // console.log("Failed event '" + e.name + "' with id '" + eID + "'")
        } else {
          // console.log("Completed event '" + e.name + "' with id '" + eID + "'")
          $timeout(function() {
            e.remove();
          }, 10000);
        }
      },
      fail: function() {
        $scope.eventFailed++;
        this.failed = true;
        this.done();
      },
      remove: function() {
        // console.log("Deleted event '" + e.name + "' with id '" + eID + "'")
        if(e.failed) {
          $scope.eventFailed--;
        }
        delete $scope.eventsPending[eID];
        $scope.eventDone--;
        $scope.eventCount--;
        return false;
      }
    };
    $scope.eventsPending[eID] = e;
    $scope.eventCount++;
    return e;
  }

  $scope.messagesDisplayed = function() {
    return $('.messages .msglist-message').length
  }

  $scope.refresh = function() {
    if ($scope.searching) {
      return $scope.refreshSearch();
    }
    var e = $scope.startEvent("Loading messages", null, "glyphicon-download");
    var url = $scope.host + 'api/v2/messages'
    if($scope.startIndex > 0) {
      url += "?start=" + $scope.startIndex + "&limit=" + $scope.itemsPerPage;
    } else {
      url += "?limit=" + $scope.itemsPerPage;
    }
    $http.get(url).success(function(data) {
      $scope.messages = data.items;
      // CC, BCC advisor


      $scope.totalMessages = data.total;
      $scope.countMessages = data.count;
      $scope.startMessages = data.start;
      e.done();
    });
  }
  $scope.refresh();

  $scope.showNewer = function() {
    if ($scope.searching) {
      $scope.searchStart -= parseInt($scope.itemsPerPage, 10) || 50;
      if ($scope.searchStart < 0) {
        $scope.searchStart = 0;
      }
      $scope.refreshSearch();
      return;
    }
    $scope.startIndex -= $scope.itemsPerPage;
    if($scope.startIndex < 0) {
      $scope.startIndex = 0
    }
    $scope.refresh();
  }

  $scope.showUpdated = function(i) {
    $scope.itemsPerPage = parseInt(i, 10);
    if(typeof(Storage) !== "undefined") {
        localStorage.setItem("itemsPerPage", $scope.itemsPerPage)
    }
    $scope.refresh();
  }

  $scope.showOlder = function() {
    if ($scope.searching) {
      $scope.searchStart += parseInt($scope.itemsPerPage, 10) || 50;
      $scope.refreshSearch();
      return;
    }
    $scope.startIndex += $scope.itemsPerPage;
    $scope.refresh();
  }

  $scope.search = function(kind, text) {
    if (!text || !String(text).trim()) {
      return;
    }
    $scope.searching = true;
    $scope.preview = null;
    $scope.searchKind = kind;
    $scope.searchedText = text;
    $scope.searchText = "";
    $scope.keepopen = false;
    $scope.searchStart = 0;
    $scope.startSearchMessages = 0
    $scope.countSearchMessages = 0
    $scope.totalSearchMessages = 0
    $scope.refreshSearch()
  }

  $scope.refreshSearch = function() {
    var url = $scope.host + 'api/v2/search?kind=' + encodeURIComponent($scope.searchKind)
      + '&query=' + encodeURIComponent($scope.searchedText)
      + '&start=' + ($scope.searchStart || 0)
      + '&limit=' + (parseInt($scope.itemsPerPage, 10) || 50);
    $http.get(url).success(function(data) {
      $scope.searchMessages = data.items || [];
      $scope.totalSearchMessages = data.total;
      $scope.countSearchMessages = data.count;
      $scope.startSearchMessages = data.start;
    });
  }

  $scope.closePreview = function() {
    $scope.preview = null;
  }

  $scope.headerValue = function(part, name) {
    if (!part || !part.Headers) {
      return "";
    }
    var values = part.Headers[name];
    if (!values) {
      for (var key in part.Headers) {
        if (key.toLowerCase() === name.toLowerCase()) {
          values = part.Headers[key];
          break;
        }
      }
    }
    return values && values.length ? values[0] : "";
  }

  $scope.isAttachmentPart = function(part) {
    if (!part) {
      return false;
    }
    var disposition = $scope.headerValue(part, "Content-Disposition").toLowerCase();
    var contentType = $scope.headerValue(part, "Content-Type").toLowerCase();
    var media = contentType.split(";")[0].trim();
    if (media.indexOf("multipart/") === 0) {
      return false;
    }
    if (disposition.indexOf("attachment") !== -1) {
      return true;
    }
    if (disposition.indexOf("inline") !== -1) {
      return false;
    }
    if (!media || media === "text/plain" || media === "text/html") {
      return false;
    }
    return true;
  }

  $scope.attachmentName = function(part) {
    var disposition = $scope.headerValue(part, "Content-Disposition");
    var contentType = $scope.headerValue(part, "Content-Type");
    var name = $scope.filenameParam(disposition) || $scope.filenameParam(contentType);
    if (name) {
      return $scope.tryDecodeMime(name);
    }
    return $scope.attachmentType(part);
  }

  $scope.filenameParam = function(value) {
    if (!value) {
      return "";
    }
    var encoded = value.match(/filename\*=(?:UTF-8|utf-8)''([^;]+)/);
    if (encoded) {
      try {
        return decodeURIComponent(encoded[1]);
      } catch (e) {
        return encoded[1];
      }
    }
    var quoted = value.match(/filename="([^"]+)"/i) || value.match(/name="([^"]+)"/i);
    if (quoted) {
      return quoted[1];
    }
    var bare = value.match(/filename=([^;]+)/i) || value.match(/name=([^;]+)/i);
    return bare ? bare[1].trim() : "";
  }

  $scope.attachmentType = function(part) {
    var contentType = $scope.headerValue(part, "Content-Type");
    return contentType ? contentType.split(";")[0].trim() : "application/octet-stream";
  }

  $scope.collectAttachments = function(message) {
    var found = [];
    if (!message || !message.MIME || !message.MIME.Parts) {
      return found;
    }
    for (var i = 0; i < message.MIME.Parts.length; i++) {
      var part = message.MIME.Parts[i];
      if (!$scope.isAttachmentPart(part)) {
        continue;
      }
      found.push({
        index: i,
        name: $scope.attachmentName(part),
        type: $scope.attachmentType(part),
        size: part.Size
      });
    }
    return found;
  }

  $scope.hasSelection = function() {
    return $(".messages :checked").length > 0 ? true : false;
  }

  $scope.selectMessage = function(message) {
    $timeout(function(){
      $scope.resizePreview();
    }, 0);
  	if($scope.cache[message.ID]) {
  		$scope.preview = $scope.cache[message.ID];
      //reflow();
  	} else {
  		$scope.preview = message;
      if (!message.attachmentParts) {
        message.attachmentParts = $scope.collectAttachments(message);
      }
      var e = $scope.startEvent("Loading message", message.ID, "glyphicon-download-alt");
	  	$http.get($scope.host + 'api/v1/messages/' + message.ID).success(function(data) {
        $scope.parseReceiver(data);
        data.attachmentParts = $scope.collectAttachments(data);
	  	  $scope.cache[message.ID] = data;

        // FIXME
        // - nested mime parts can't be downloaded

        data.$cidMap = {};
        if(data.MIME && data.MIME.Parts.length) {
          for(p in data.MIME.Parts) {
            for(h in data.MIME.Parts[p].Headers) {
              if(h.toLowerCase() == "content-id") {
                cid = data.MIME.Parts[p].Headers[h][0]
                cid = cid.substr(1,cid.length-2)
                data.$cidMap[cid] = "api/v1/messages/" + message.ID + "/mime/part/" + p + "/download"
              }
            }
          }
        }
        console.log(data.$cidMap)
        // TODO
        // - scan HTML parts for elements containing CID URI and replace

        h = $scope.getMessageHTML(data)
        for(c in data.$cidMap) {
	  str = "cid:" + c;
	  pat = str.replace(/([.*+?^=!:${}()|\[\]\/\\])/g, "\\$1");
          h = h.replace(new RegExp(pat, 'g'), data.$cidMap[c])
        }
	      data.previewHTML = $sce.trustAsHtml(h);
  		  $scope.preview = data;
  		  preview = $scope.cache[message.ID];
        //reflow();
        e.done();
	    });
	   }
  }

  $scope.toggleHeaders = function(val) {
    $scope.previewAllHeaders = val;
    $timeout(function(){
      $scope.resizePreview();
    }, 0);
    var t = window.setInterval(function() {
      if(val) {
        if($('#hide-headers').length) {
          window.clearInterval(t);
          //reflow();
        }
      } else {
        if($('#show-headers').length) {
          window.clearInterval(t);
          //reflow();
        }
      }
    }, 10);
  }

  $scope.fileSize = function(bytes) {
    return filesize(bytes)
  }

  $scope.tryDecodeContent = function(message) {
    var charset = "UTF-8"
    if(message.Content.Headers["Content-Type"][0]) {
      // TODO
    }

    var content = message.Content.Body;
    var contentTransferEncoding = message.Content.Headers["Content-Transfer-Encoding"][0];

    if(contentTransferEncoding) {
      switch (contentTransferEncoding.toLowerCase()) {
        case 'quoted-printable':
          content = content.replace(/=[\r\n]+/gm,"");
          content = unescapeFromQuotedPrintableWithoutRFC2047(content, charset);
          break;
        case 'base64':
          // remove line endings to give original base64-encoded string
          content = content.replace(/\r?\n|\r/gm,"");
          content = unescapeFromBase64(content, charset);
          break;
      }
    }

    return content;
  }

  $scope.formatMessagePlain = function(message) {
    var body = $scope.getMessagePlain(message);
    var escaped = $scope.escapeHtml(body);
    var formatted = escaped.replace(/(https?:\/\/)([-[\]A-Za-z0-9._~:/?#@!$()*+,;=%]|&amp;|&#39;)+/g, '<a href="$&" target="_blank">$&</a>');
    return $sce.trustAsHtml(formatted);
  }

  $scope.escapeHtml = function(html) {
    var entityMap = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;'
    };
    return html.replace(/[&<>"']/g, function (s) {
      return entityMap[s];
    });
  }

  $scope.getMessagePlain = function(message) {
    if (message.Content.Headers && message.Content.Headers["Content-Type"] && message.Content.Headers["Content-Type"][0].match("text/plain")) {
      return $scope.tryDecode(message.Content);
    }
    var l = $scope.findMatchingMIME(message, "text/plain");
    if(l != null && l !== "undefined") {
      return $scope.tryDecode(l);
    }
    return message.Content.Body;
  }

  $scope.findMatchingMIME = function(part, mime) {
    // TODO cache results
    if(part.MIME) {
      for(var p in part.MIME.Parts) {
        if("Content-Type" in part.MIME.Parts[p].Headers) {
          if(part.MIME.Parts[p].Headers["Content-Type"].length > 0) {
            if(part.MIME.Parts[p].Headers["Content-Type"][0].match(mime + ";?.*")) {
              return part.MIME.Parts[p];
            } else if (part.MIME.Parts[p].Headers["Content-Type"][0].match(/multipart\/.*/)) {
              var f = $scope.findMatchingMIME(part.MIME.Parts[p], mime);
              if(f != null) {
                return f;
              }
            }
          }
        }
      }
    }
    return null;
  }
  $scope.hasHTML = function(message) {
    // TODO cache this
    for(var header in message.Content.Headers) {
      if(header.toLowerCase() == 'content-type') {
        if(message.Content.Headers[header][0].match("text/html")) {
          return true
        }
      }
    }

    var l = $scope.findMatchingMIME(message, "text/html");
    if(l != null && l !== "undefined") {
      return true
    }
    return false;
  }
  $scope.getMessageHTML = function(message) {
    console.log(message);
    for(var header in message.Content.Headers) {
      if(header.toLowerCase() == 'content-type') {
        if(message.Content.Headers[header][0].match("text/html")) {
          return $scope.tryDecode(message.Content);
        }
      }
    }

    var l = $scope.findMatchingMIME(message, "text/html");
    if(l != null && l !== "undefined") {
      return $scope.tryDecode(l);
    }
  	return "<HTML not found>";
	}

  $scope.tryDecode = function(l){
    if(l.Headers && l.Headers["Content-Type"] && l.Headers["Content-Transfer-Encoding"]){
      return $scope.tryDecodeContent({Content:l});
    }else{
      return l.Body;
    }
  };
  $scope.date = function(timestamp) {
  	return (new Date(timestamp)).toString();
  };

  $scope.deleteAll = function() {
  	$('#confirm-delete-all').modal('show');
  }

  window.mailtrapClearAll = function() {
    var root = document.querySelector('[ng-controller="MailCtrl"]') || document.body;
    var scope = angular.element(root).scope();
    if (!scope) {
      return;
    }
    scope.$apply(function() {
      scope.deleteAll();
    });
  }

  $scope.releaseOne = function(message) {
    $scope.releasing = message;

    $http.get($scope.host + 'api/v2/outgoing-smtp').success(function(data) {
      $scope.outgoingSMTP = data;
      $('#release-one').modal('show');
    })
  }
  $scope.confirmReleaseMessage = function() {
    $('#release-one').modal('hide');
    var message = $scope.releasing;
    $scope.releasing = null;

    var e = $scope.startEvent("Releasing message", message.ID, "glyphicon-share");

    if($('#release-message-outgoing').val().length > 0) {
      authcfg = {
        name: $('#release-message-outgoing').val(),
        email: $('#release-message-email').val(),
      }
    } else {
      authcfg = {
        email: $('#release-message-email').val(),
        host: $('#release-message-smtp-host').val(),
        port: $('#release-message-smtp-port').val(),
        mechanism: $('#release-message-smtp-mechanism').val(),
        username: $('#release-message-smtp-username').val(),
        password: $('#release-message-smtp-password').val(),
        save: $('#release-message-save').is(":checked") ? true : false,
        name: $('#release-message-server-name').val(),
      }
    }

    $http.post($scope.host + 'api/v1/messages/' + message.ID + '/release', authcfg).success(function() {
      e.done();
    }).error(function(err) {
      e.fail();
      e.error = err;
    });
  }

  $scope.getSource = function(message) {
  	var source = "";
  	$.each(message.Content.Headers, function(k, v) {
  		source += k + ": " + v + "\n";
  	});
	source += "\n";
	source += message.Content.Body;
	return source;
  }

  $scope.deleteAllConfirm = function() {
  	$('#confirm-delete-all').modal('hide');
    var e = $scope.startEvent("Deleting all messages", null, "glyphicon-remove-circle");
  	$http.delete($scope.host + 'api/v1/messages').success(function() {
  		$scope.refresh();
  		$scope.preview = null;
      e.done()
  	});
  }

  $scope.deleteOne = function(message) {
    var e = $scope.startEvent("Deleting message", message.ID, "glyphicon-remove");
  	$http.delete($scope.host + 'api/v1/messages/' + message.ID).success(function() {
  		if($scope.preview._id == message._id) $scope.preview = null;
  		$scope.refresh();
      e.done();
  	});
  }
});
