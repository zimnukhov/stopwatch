$(function() {
    var stopwatchPage = true;
    var startHour = window.DayStartHour;

    var getDayStart = function(date) {
        if (date.getHours() < startHour) {
            date = new Date(date.getTime() - 86400000);
        }
        date.setHours(startHour);
        date.setMinutes(0);
        date.setSeconds(0);
        return date;
    }

    var urlSplit = location.pathname.split("/");
    var pageDayStart = null;
    if (m = urlSplit[urlSplit.length-2].match(/^(\d{4})-(\d{2})-(\d{2})$/)) {
        pageDayStart = new Date(m[1], m[2]-1, m[3], startHour, 0, 0);
        stopwatchPage = false;
    }
    else {
        pageDayStart = getDayStart(new Date());
    }

    var getDurationString = function(microseconds) {
        var micro = parseInt(microseconds % 1000);
        var seconds = microseconds / 1000;
        var hours = parseInt(seconds / 3600);
        seconds %= 3600;
        var minutes = parseInt(seconds / 60);
        seconds %= 60;

        var timeStr = hours + ":";

        if (minutes < 10) {
            timeStr += "0";
        }
        timeStr += minutes + ":";

        if (seconds < 10) {
            timeStr += "0";
        }
        timeStr += parseInt(seconds);
        var microStr = micro.toString();
        var padChars = 3 - microStr.length;
        for (var i = 0; i < padChars; i++) {
            microStr = "0" + microStr;
        }
        timeStr +=  "." + microStr;
        return timeStr;
    }
    var displayTime = function(microseconds) {
        $("#time").text(getDurationString(microseconds));
    }

    var running = false;
    var updateInterval = null;
    var elapsedTime = 0; // time from previous sessions
    var sessionStartTS = 0;
    var button = $("#toggle");

    var toggle = function() {
        if (running) {
            clearInterval(updateInterval);
            button.removeClass("stop").addClass("start").text("start");
            elapsedTime += new Date().getTime() - sessionStartTS
        }
        else {
            sessionStartTS = new Date().getTime();
            updateInterval = setInterval(function() {
                var ts = new Date().getTime();
                displayTime(elapsedTime + ts - sessionStartTS);
            }, 43);
            button.removeClass("start").addClass("stop").text("stop");
        }
        running = !running;
    }

    var request = function(action) {
        $.ajax({
            url: StopwatchPrefix + "/" + action,
            dataType: "json",
            success: function(response) {
                elapsedTime = response.time;
                displayTime(response.time);
                if (response.running != running) {
                    toggle();
                    if (!response.running) {
                        redrawSessions();
                    }
                }
            },
        });
    }

    if (stopwatchPage) {
        request("time");
    }

    var redrawSessions = function() {
        var url = StopwatchPrefix + "/sessions";
        if (!stopwatchPage) {
            url += "?time=" + pageDayStart.getTime()
        }

        $.ajax({
            url: url,
            dataType: "json",
            success: function(sessions) {
                var timeline = $("#timeline");
                timeline.children("div").remove();

                dayStart = pageDayStart;
                var endTime = Math.min(new Date().getTime() + 100000, dayStart.getTime() + 86400000);
                var zeroPoint = dayStart.getTime();

                for (var ts = dayStart.getTime(); ts < endTime; ts += 3600000) {
                    var date = new Date(ts);
                    var tickText = date.getHours().toString() + ":";
                    var minutes = date.getMinutes();
                    if (minutes < 10) {
                        tickText += "0";
                    }
                    tickText += minutes.toString();
                    var left = 100 * (ts - zeroPoint) / (endTime - dayStart);
                    if (left <= 95) {
                        $("<div>").addClass("tick-legend").text(tickText).css("left", left + "%").appendTo(timeline);
                    }
                    $("<div>").addClass("tick").css("left", left + "%").appendTo(timeline);
                }

                var sessionTS2TimelinePercent = function(start, end) {
                    return [
                        100 * (start - zeroPoint) / (endTime - zeroPoint),
                        100 * (end - start) / (endTime - zeroPoint)
                    ];
                }

                for(var i=0; i < sessions.length; i++) {
                    var pixData = sessionTS2TimelinePercent(sessions[i].start, sessions[i].end);
                    console.log("left " + pixData[0]);
                    console.log("width " + pixData[1]);
                    console.log("endtime " + endTime);
                    console.log("zero point " + zeroPoint);
                    var titleText = new Date(sessions[i].start) + " - " + new Date(sessions[i].end);
                    titleText += " duration: " + getDurationString(sessions[i].end - sessions[i].start);

                    $("<div>").addClass("work")
                        .css("width", pixData[1] + "%")
                        .css("left", pixData[0] + "%")
                        .attr("title", titleText)
                        .appendTo(timeline);
                }
            },
        });
    }
    redrawSessions();

    if (stopwatchPage) {
        $("#toggle").click(function(){
            if (running) {
                request("stop");
            }
            else {
                request("start");
            }
        });
        // subscribe to updates over websocket:
        (function() {
            var updatesSocket = new WebSocket("ws://" + location.host + StopwatchPrefix + "/updates");

            updatesSocket.onmessage = function(event) {
                request("time");
            }
        })()
    }
});
