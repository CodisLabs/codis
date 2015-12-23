/**
 * SVG map parser. 
 * This file requires data.js.
 */

/*global document, Highcharts, jQuery, $ */
(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (H) {

	'use strict';

	var each = H.each;

	H.wrap(H.Data.prototype, 'init', function (proceed, options) {
		proceed.call(this, options);

		if (options.svg) {
			this.loadSVG();
		}
	});

	H.extend(H.Data.prototype, {
		/**
		 * Parse an SVG path into a simplified array that Highcharts can read
		 */
		pathToArray: function (path, matrix) {
			var i = 0,
				position = 0,
				point,
				positions,
				fixedPoint = [0, 0],
				startPoint = [0, 0],
				isRelative,
				isString,
				operator,
				matrixTransform = function (p, m) {
					return [
						m.a * p[0] + m.c * p[1] + m.e,
						m.b * p[0] + m.d * p[1] + m.f
					];
				};

			path = path
				// Scientific notation
				.replace(/[0-9]+e-?[0-9]+/g, function (a) {
					return +a; // cast to number
				})
				// Move letters apart
				.replace(/([A-Za-z])/g, ' $1 ')
				// Add space before minus
				.replace(/-/g, ' -')
				// Trim
				.replace(/^\s*/, '').replace(/\s*$/, '')
				// Remove newlines, tabs etc
				.replace(/\s+/g, ' ')
			
				// Split on spaces, minus and commas
				.split(/[ ,]+/);
			
			// Blank path
			if (path.length === 1) {
				return [];	
			}

			// Real path
			for (i = 0; i < path.length; i++) {
				isString = /[a-zA-Z]/.test(path[i]);
				
				// Handle strings
				if (isString) {
					operator = path[i];
					positions = 2;
					
					// Curves have six positions
					if (operator === 'c' || operator === 'C') {
						positions = 6;
					}

					// When moving after a closed subpath, start again from previous subpath's starting point
					if (operator === 'm') {
						startPoint = [parseFloat(path[i + 1]) + startPoint[0], parseFloat(path[i + 2]) + startPoint[1]];
					} else if (operator === 'M') {
						startPoint = [parseFloat(path[i + 1]), parseFloat(path[i + 2])];
					}
					
					// Enter or exit relative mode
					if (operator === 'm' || operator === 'l' || operator === 'c') {
						path[i] = operator.toUpperCase();
						isRelative = true;
					} else if (operator === 'M' || operator === 'L' || operator === 'C') {
						isRelative = false;

					
					// Horizontal and vertical line to
					} else if (operator === 'h') {
						isRelative = true;
						path[i] = 'L';
						path.splice(i + 2, 0, 0);
					} else if (operator === 'v') {
						isRelative = true;
						path[i] = 'L';
						path.splice(i + 1, 0, 0);
					} else if (operator === 's') {
						isRelative = true;
						path[i] = 'L';
						path.splice(i + 1, 2);
					} else if (operator === 'S') {
						isRelative = false;
						path[i] = 'L';
						path.splice(i + 1, 2);
					} else if (operator === 'H' || operator === 'h') {
						isRelative = false;
						path[i] = 'L';
						path.splice(i + 2, 0, fixedPoint[1]);
					} else if (operator === 'V' || operator === 'v') {
						isRelative = false;
						path[i] = 'L';
						path.splice(i + 1, 0, fixedPoint[0]);
					} else if (operator === 'z' || operator === 'Z') {
						fixedPoint = startPoint;
					}
				
				// Handle numbers
				} else {
					path[i] = parseFloat(path[i]);
					if (isRelative) {
						path[i] += fixedPoint[position % 2];
					
					} 

					if (position % 2 === 1) { // y
						// only translate absolute points or initial moveTo
						if (matrix && (!isRelative || (operator === 'm' && i < 3))) {
							point = matrixTransform([path[i - 1], path[i]], matrix);
							path[i - 1] = point[0];
							path[i] = point[1];
						}

						// Add it
						//path[i - 1] = Math.round(path[i - 1] * 100) / 100; // x
						//path[i] = Math.round(path[i] * 100) / 100; // y
					}	
					
					
					// Reset to zero position (x/y switching)
					if (position === positions - 1) {
						// Set the fixed point for the next pair
						fixedPoint = [path[i - 1], path[i]];
					
						position = 0;
					} else {
						position += 1;
					}

				}
			}

			// Handle polygon points
			if (typeof path[0] === 'number' && path.length >= 4) {
				path.unshift('M');
				path.splice(3, 0, 'L');
			}
			return path;
		},

		/**
		 * Join the path back to a string for compression
		 */
		pathToString: function (arr) {
			each(arr, function (point) {
				var path = point.path;

				// Join all by commas
				path = path.join(',');

				// Remove commas next to a letter
				path = path.replace(/,?([a-zA-Z]),?/g, '$1');
				
				// Reinsert
				point.path = path;
			});

			return arr;
			//return path.join(',')
		},

		/**
		 * Scale the path to fit within a given box and round all numbers
		 */
		roundPaths: function (arr, scale) {
			var mapProto = Highcharts.seriesTypes.map.prototype,
				fakeSeries,
				origSize,
				transA;

			fakeSeries = {
				xAxis: {
					//min: arr.minX,
					//len: scale,
					translate: Highcharts.Axis.prototype.translate,
					options: {},
					minPixelPadding: 0
					//transA: transA
				}, 
				yAxis: {
					//min: (arr.minY + scale) / transA,
					//len: scale,
					translate: Highcharts.Axis.prototype.translate,
					options: {},
					minPixelPadding: 0
					//transA: transA
				}
			};
			
			// Borrow the map series type's getBox method
			mapProto.getBox.call(fakeSeries, arr);

			origSize = Math.max(fakeSeries.maxX - fakeSeries.minX, fakeSeries.maxY - fakeSeries.minY);
			scale = scale || 1000;
			transA = scale / origSize;

			fakeSeries.xAxis.transA = fakeSeries.yAxis.transA = transA;
			fakeSeries.xAxis.len = fakeSeries.yAxis.len = scale;
			fakeSeries.xAxis.min = fakeSeries.minX;
			fakeSeries.yAxis.min = (fakeSeries.minY + scale) / transA;

			each(arr, function (point) {

				var i,
					path;
				point.path = path = mapProto.translatePath.call(fakeSeries, point.path, true);
				i = path.length;
				while (i--) {
					if (typeof path[i] === 'number') {
						path[i] = Math.round(path[i]);
					}
				}
				delete point._foundBox;

			});

			return arr;
		},
		
		/**
		 * Load an SVG file and extract the paths
		 * @param {Object} url
		 */
		loadSVG: function () {
			
			var data = this,
				options = this.options;

			function getPathLikeChildren(parent) {
				return Array.prototype.slice.call(parent.getElementsByTagName('path'))
					.concat(Array.prototype.slice.call(parent.getElementsByTagName('polygon')))
					.concat(Array.prototype.slice.call(parent.getElementsByTagName('rect')));
			}

			function getPathDefinition(node) {
				if (node.nodeName === 'path') {
					return node.getAttribute('d');
				}
				if (node.nodeName === 'polygon') {
					return node.getAttribute('points');
				}
				if (node.nodeName === 'rect') {
					var x = +node.getAttribute('x'),
						y = +node.getAttribute('y'),
						w = +node.getAttribute('width'),
						h = +node.getAttribute('height');

					// Return polygon definition
					return [x, y, x + w, y, x + w, y + h, x, y + h, x, y].join(' ');
				}
			}

			function getTranslate(elem) {
				var ctm = elem.getCTM();
				if (!isNaN(ctm.f)) {
					return ctm;
				}
			}

			
			function getName(elem) {
				var desc = elem.getElementsByTagName('desc'),
					nameTag = desc[0] && desc[0].getElementsByTagName('name'),
					name = nameTag && nameTag[0] && nameTag[0].innerText;

				return name || elem.getAttribute('inkscape:label') || elem.getAttribute('id') || elem.getAttribute('class');
			}

			function hasFill(elem) {
				return !/fill[\s]?\:[\s]?none/.test(elem.getAttribute('style')) && elem.getAttribute('fill') !== 'none';
			}

			function handleSVG(xml) {

				var arr = [],
					currentParent,
					allPaths,
					commonLineage,
					lastCommonAncestor,
					handleGroups;

				// Make a hidden frame where the SVG is rendered
				data.$frame = data.$frame || $('<div>')
					.css({
						position: 'absolute', // https://bugzilla.mozilla.org/show_bug.cgi?id=756985
						top: '-9999em'
					})
					.appendTo($(document.body));
				data.$frame.html(xml);
				xml = $('svg', data.$frame)[0];

				xml.removeAttribute('viewBox');
					

				allPaths = getPathLikeChildren(xml);
					
				// Skip clip paths
				each(['defs', 'clipPath'], function (nodeName) {
					each(xml.getElementsByTagName(nodeName), function (parent) {
						each(parent.getElementsByTagName('path'), function (path) {
							path.skip = true;
						});
					});
				});
				
				// If not all paths belong to the same group, handle groups
				each(allPaths, function (path, i) {
					if (!path.skip) {
						var itemLineage = [],
							parentNode,
							j;
						
						if (i > 0 && path.parentNode !== currentParent) {
							handleGroups = true;
						}
						currentParent = path.parentNode;
						
						// Handle common lineage
						parentNode = path;
						while (parentNode) {
							itemLineage.push(parentNode);
							parentNode = parentNode.parentNode;
						}
						itemLineage.reverse();
						
						if (!commonLineage) {
							commonLineage = itemLineage; // first iteration
						} else {
							for (j = 0; j < commonLineage.length; j++) {
								if (commonLineage[j] !== itemLineage[j]) {
									commonLineage = commonLineage.slice(0, j);
								}
							}
						}
					}
				});
				lastCommonAncestor = commonLineage[commonLineage.length - 1];
				
				// Iterate groups to find sub paths
				if (handleGroups) {
					each(lastCommonAncestor.getElementsByTagName('g'), function (g) {
						var groupPath = [],
							pathHasFill;
						
						each(getPathLikeChildren(g), function (path) {
							if (!path.skip) {
								groupPath = groupPath.concat(
									data.pathToArray(getPathDefinition(path), getTranslate(path))
								);

								if (hasFill(path)) {
									pathHasFill = true;
								}
								
								path.skip = true;
							}
						});
						arr.push({
							name: getName(g),
							path: groupPath,
							hasFill: pathHasFill
						});
					});
				}
				
				// Iterate the remaining paths that are not parts of groups
				each(allPaths, function (path) {
					if (!path.skip) {
						arr.push({
							name: getName(path),
							path: data.pathToArray(getPathDefinition(path), getTranslate(path)),
							hasFill: hasFill(path)
						});
					}			
				});

				// Round off to compress
				data.roundPaths(arr);
				
				// Do the callback
				options.complete({
					series: [{
						data: arr
					}]
				});
			}
			
			if (options.svg.indexOf('<svg') !== -1) {
				handleSVG(options.svg);
			} else {
				jQuery.ajax({
					url: options.svg,
					dataType: 'text',
					success: handleSVG
				});
			}
		}
	});
}));
