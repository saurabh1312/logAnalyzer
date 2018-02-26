function loadGraphData() {
    var field = $("#fieldSelect").val();
    var url = "/fetchData/" + field;
    var ykey = "value";
    var ylabel = "Value";

    $("#graph_div").empty();

    $.getJSON(url, function (json) {
        Morris.Bar({
            element : "graph_div",
            data : json,
            xkey : "field",
            ykeys: [ykey],
            labels: [ylabel]
        });
    });
}

function loadFile() {
    var formData = new FormData();
    formData.append("logFile", $("#logFile").prop('files')[0]);

    $.ajax({
        url: "/uploadFile",
        data: formData,
        type: 'POST',
        contentType: false,
        processData: false,
        success : function(data) {
            $("#fieldSelect").val("url_count(url)_api");
            loadGraphData()
        }
    });
}