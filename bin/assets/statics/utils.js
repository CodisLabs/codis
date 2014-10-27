// Hash any string into an integer value
function hashCode(str) {
    var hash = 0;
    for (var i = 0; i < str.length; i++) {
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }
    return hash;
}

// Convert an int to hexadecimal with a max length
// of six characters.
function intToARGB(i) {
    var h = ((i>>24)&0xFF).toString(16) +
            ((i>>16)&0xFF).toString(16) +
            ((i>>8)&0xFF).toString(16) +
            (i&0xFF).toString(16);
    return h.substring(0, 6);
}

function tmpl(tmpl_name) {
    if ( !tmpl.tmpl_cache ) {
        tmpl.tmpl_cache = {};
    }

    if ( ! tmpl.tmpl_cache[tmpl_name] ) {
        var tmpl_dir = '/admin/templates';
        var tmpl_url = tmpl_dir + '/' + tmpl_name + '.html';

        var tmpl_string;
        $.ajax({
            url: tmpl_url,
            method: 'GET',
            async: false,
            success: function(data) {
                tmpl_string = data;
            }
        });

        tmpl.tmpl_cache[tmpl_name] = _.template(tmpl_string);
    }

    return tmpl.tmpl_cache[tmpl_name];
}

