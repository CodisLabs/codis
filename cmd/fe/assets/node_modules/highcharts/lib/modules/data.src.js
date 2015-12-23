/**
 * @license Highcharts JS v4.1.10 (2015-12-07)
 * Data module
 *
 * (c) 2012-2014 Torstein Honsi
 *
 * License: www.highcharts.com/license
 */

/*global jQuery */
(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (Highcharts) {
	
	// Utilities
	var each = Highcharts.each,
		pick = Highcharts.pick,
		inArray = Highcharts.inArray,
		splat = Highcharts.splat,
		SeriesBuilder;
	
	
	// The Data constructor
	var Data = function (dataOptions, chartOptions) {
		this.init(dataOptions, chartOptions);
	};
	
	// Set the prototype properties
	Highcharts.extend(Data.prototype, {
		
		/**
		 * Initialize the Data object with the given options
		 */
		init: function (options, chartOptions) {
			this.options = options;
			this.chartOptions = chartOptions;
			this.columns = options.columns || this.rowsToColumns(options.rows) || [];
			this.firstRowAsNames = pick(options.firstRowAsNames, true);
			this.decimalRegex = options.decimalPoint && new RegExp('^(-?[0-9]+)' + options.decimalPoint + '([0-9]+)$');

			// This is a two-dimensional array holding the raw, trimmed string values
			// with the same organisation as the columns array. It makes it possible
			// for example to revert from interpreted timestamps to string-based
			// categories.
			this.rawColumns = [];

			// No need to parse or interpret anything
			if (this.columns.length) {
				this.dataFound();

			// Parse and interpret
			} else {

				// Parse a CSV string if options.csv is given
				this.parseCSV();
				
				// Parse a HTML table if options.table is given
				this.parseTable();

				// Parse a Google Spreadsheet 
				this.parseGoogleSpreadsheet();	
			}

		},

		/**
		 * Get the column distribution. For example, a line series takes a single column for 
		 * Y values. A range series takes two columns for low and high values respectively,
		 * and an OHLC series takes four columns.
		 */
		getColumnDistribution: function () {
			var chartOptions = this.chartOptions,
				options = this.options,
				xColumns = [],
				getValueCount = function (type) {
					return (Highcharts.seriesTypes[type || 'line'].prototype.pointArrayMap || [0]).length;
				},
				getPointArrayMap = function (type) {
					return Highcharts.seriesTypes[type || 'line'].prototype.pointArrayMap;
				},
				globalType = chartOptions && chartOptions.chart && chartOptions.chart.type,
				individualCounts = [],
				seriesBuilders = [],
				seriesIndex = 0,
				i;

			each((chartOptions && chartOptions.series) || [], function (series) {
				individualCounts.push(getValueCount(series.type || globalType));
			});

			// Collect the x-column indexes from seriesMapping
			each((options && options.seriesMapping) || [], function (mapping) {
				xColumns.push(mapping.x || 0);
			});

			// If there are no defined series with x-columns, use the first column as x column
			if (xColumns.length === 0) {
				xColumns.push(0);
			}

			// Loop all seriesMappings and constructs SeriesBuilders from
			// the mapping options.
			each((options && options.seriesMapping) || [], function (mapping) {
				var builder = new SeriesBuilder(),
					name,
					numberOfValueColumnsNeeded = individualCounts[seriesIndex] || getValueCount(globalType),
					seriesArr = (chartOptions && chartOptions.series) || [],
					series = seriesArr[seriesIndex] || {},
					pointArrayMap = getPointArrayMap(series.type || globalType) || ['y'];

				// Add an x reader from the x property or from an undefined column
				// if the property is not set. It will then be auto populated later.
				builder.addColumnReader(mapping.x, 'x');

				// Add all column mappings
				for (name in mapping) {
					if (mapping.hasOwnProperty(name) && name !== 'x') {
						builder.addColumnReader(mapping[name], name);
					}
				}

				// Add missing columns
				for (i = 0; i < numberOfValueColumnsNeeded; i++) {
					if (!builder.hasReader(pointArrayMap[i])) {
						//builder.addNextColumnReader(pointArrayMap[i]);
						// Create and add a column reader for the next free column index
						builder.addColumnReader(undefined, pointArrayMap[i]);
					}
				}

				seriesBuilders.push(builder);
				seriesIndex++;
			});

			var globalPointArrayMap = getPointArrayMap(globalType);
			if (globalPointArrayMap === undefined) {
				globalPointArrayMap = ['y'];
			}

			this.valueCount = {
				global: getValueCount(globalType),
				xColumns: xColumns,
				individual: individualCounts,
				seriesBuilders: seriesBuilders,
				globalPointArrayMap: globalPointArrayMap
			};
		},

		/**
		 * When the data is parsed into columns, either by CSV, table, GS or direct input,
		 * continue with other operations.
		 */
		dataFound: function () {
			
			if (this.options.switchRowsAndColumns) {
				this.columns = this.rowsToColumns(this.columns);
			}

			// Interpret the info about series and columns
			this.getColumnDistribution();

			// Interpret the values into right types
			this.parseTypes();
			
			// Handle columns if a handleColumns callback is given
			if (this.parsed() !== false) {
			
				// Complete if a complete callback is given
				this.complete();
			}
			
		},
		
		/**
		 * Parse a CSV input string
		 */
		parseCSV: function () {
			var self = this,
				options = this.options,
				csv = options.csv,
				columns = this.columns,
				startRow = options.startRow || 0,
				endRow = options.endRow || Number.MAX_VALUE,
				startColumn = options.startColumn || 0,
				endColumn = options.endColumn || Number.MAX_VALUE,
				itemDelimiter,
				lines,
				activeRowNo = 0;
				
			if (csv) {
				
				lines = csv
					.replace(/\r\n/g, '\n') // Unix
					.replace(/\r/g, '\n') // Mac
					.split(options.lineDelimiter || '\n');

				itemDelimiter = options.itemDelimiter || (csv.indexOf('\t') !== -1 ? '\t' : ',');
				
				each(lines, function (line, rowNo) {
					var trimmed = self.trim(line),
						isComment = trimmed.indexOf('#') === 0,
						isBlank = trimmed === '',
						items;
					
					if (rowNo >= startRow && rowNo <= endRow && !isComment && !isBlank) {
						items = line.split(itemDelimiter);
						each(items, function (item, colNo) {
							if (colNo >= startColumn && colNo <= endColumn) {
								if (!columns[colNo - startColumn]) {
									columns[colNo - startColumn] = [];					
								}
								
								columns[colNo - startColumn][activeRowNo] = item;
							}
						});
						activeRowNo += 1;
					}
				});

				this.dataFound();
			}
		},
		
		/**
		 * Parse a HTML table
		 */
		parseTable: function () {
			var options = this.options,
				table = options.table,
				columns = this.columns,
				startRow = options.startRow || 0,
				endRow = options.endRow || Number.MAX_VALUE,
				startColumn = options.startColumn || 0,
				endColumn = options.endColumn || Number.MAX_VALUE;

			if (table) {
				
				if (typeof table === 'string') {
					table = document.getElementById(table);
				}
				
				each(table.getElementsByTagName('tr'), function (tr, rowNo) {
					if (rowNo >= startRow && rowNo <= endRow) {
						each(tr.children, function (item, colNo) {
							if ((item.tagName === 'TD' || item.tagName === 'TH') && colNo >= startColumn && colNo <= endColumn) {
								if (!columns[colNo - startColumn]) {
									columns[colNo - startColumn] = [];					
								}
								
								columns[colNo - startColumn][rowNo - startRow] = item.innerHTML;
							}
						});
					}
				});

				this.dataFound(); // continue
			}
		},

		/**
		 */
		parseGoogleSpreadsheet: function () {
			var self = this,
				options = this.options,
				googleSpreadsheetKey = options.googleSpreadsheetKey,
				columns = this.columns,
				startRow = options.startRow || 0,
				endRow = options.endRow || Number.MAX_VALUE,
				startColumn = options.startColumn || 0,
				endColumn = options.endColumn || Number.MAX_VALUE,
				gr, // google row
				gc; // google column

			if (googleSpreadsheetKey) {
				jQuery.ajax({
					dataType: 'json', 
					url: 'https://spreadsheets.google.com/feeds/cells/' + 
						googleSpreadsheetKey + '/' + (options.googleSpreadsheetWorksheet || 'od6') +
						'/public/values?alt=json-in-script&callback=?',
					error: options.error,
					success: function (json) {
						// Prepare the data from the spreadsheat
						var cells = json.feed.entry,
							cell,
							cellCount = cells.length,
							colCount = 0,
							rowCount = 0,
							i;
					
						// First, find the total number of columns and rows that 
						// are actually filled with data
						for (i = 0; i < cellCount; i++) {
							cell = cells[i];
							colCount = Math.max(colCount, cell.gs$cell.col);
							rowCount = Math.max(rowCount, cell.gs$cell.row);			
						}
					
						// Set up arrays containing the column data
						for (i = 0; i < colCount; i++) {
							if (i >= startColumn && i <= endColumn) {
								// Create new columns with the length of either end-start or rowCount
								columns[i - startColumn] = [];

								// Setting the length to avoid jslint warning
								columns[i - startColumn].length = Math.min(rowCount, endRow - startRow);
							}
						}
						
						// Loop over the cells and assign the value to the right
						// place in the column arrays
						for (i = 0; i < cellCount; i++) {
							cell = cells[i];
							gr = cell.gs$cell.row - 1; // rows start at 1
							gc = cell.gs$cell.col - 1; // columns start at 1

							// If both row and col falls inside start and end
							// set the transposed cell value in the newly created columns
							if (gc >= startColumn && gc <= endColumn &&
								gr >= startRow && gr <= endRow) {
								columns[gc - startColumn][gr - startRow] = cell.content.$t;
							}
						}
						self.dataFound();
					}
				});
			}
		},
		
		/**
		 * Trim a string from whitespace
		 */
		trim: function (str, inside) {
			if (typeof str === 'string') {
				str = str.replace(/^\s+|\s+$/g, '');

				// Clear white space insdie the string, like thousands separators
				if (inside && /^[0-9\s]+$/.test(str)) { 
					str = str.replace(/\s/g, '');
				}

				if (this.decimalRegex) {
					str = str.replace(this.decimalRegex, '$1.$2');
				}
			}
			return str;
		},
		
		/**
		 * Parse numeric cells in to number types and date types in to true dates.
		 */
		parseTypes: function () {
			var columns = this.columns,
				col = columns.length;

			while (col--) {
				this.parseColumn(columns[col], col);
			}

		},

		/**
		 * Parse a single column. Set properties like .isDatetime and .isNumeric.
		 */
		parseColumn: function (column, col) {
			var rawColumns = this.rawColumns,
				columns = this.columns, 
				row = column.length,
				val,
				floatVal,
				trimVal,
				trimInsideVal,
				firstRowAsNames = this.firstRowAsNames,
				isXColumn = inArray(col, this.valueCount.xColumns) !== -1,
				dateVal,
				backup = [],
				diff,
				chartOptions = this.chartOptions,
				descending,
				columnTypes = this.options.columnTypes || [],
				columnType = columnTypes[col],
				forceCategory = isXColumn && ((chartOptions && chartOptions.xAxis && splat(chartOptions.xAxis)[0].type === 'category') || columnType === 'string');
			
			if (!rawColumns[col]) {
				rawColumns[col] = [];
			}
			while (row--) {
				val = backup[row] || column[row];
				
				trimVal = this.trim(val);
				trimInsideVal = this.trim(val, true);
				floatVal = parseFloat(trimInsideVal);

				// Set it the first time
				if (rawColumns[col][row] === undefined) {
					rawColumns[col][row] = trimVal;
				}
				
				// Disable number or date parsing by setting the X axis type to category
				if (forceCategory || (row === 0 && firstRowAsNames)) {
					column[row] = trimVal;

				} else if (+trimInsideVal === floatVal) { // is numeric
				
					column[row] = floatVal;
					
					// If the number is greater than milliseconds in a year, assume datetime
					if (floatVal > 365 * 24 * 3600 * 1000 && columnType !== 'float') {
						column.isDatetime = true;
					} else {
						column.isNumeric = true;
					}

					if (column[row + 1] !== undefined) {
						descending = floatVal > column[row + 1];
					}
				
				// String, continue to determine if it is a date string or really a string
				} else {
					dateVal = this.parseDate(val);
					// Only allow parsing of dates if this column is an x-column
					if (isXColumn && typeof dateVal === 'number' && !isNaN(dateVal) && columnType !== 'float') { // is date
						backup[row] = val; 
						column[row] = dateVal;
						column.isDatetime = true;

						// Check if the dates are uniformly descending or ascending. If they 
						// are not, chances are that they are a different time format, so check
						// for alternative.
						if (column[row + 1] !== undefined) {
							diff = dateVal > column[row + 1];
							if (diff !== descending && descending !== undefined) {
								if (this.alternativeFormat) {
									this.dateFormat = this.alternativeFormat;
									row = column.length;
									this.alternativeFormat = this.dateFormats[this.dateFormat].alternative;
								} else {
									column.unsorted = true;
								}
							}
							descending = diff;
						}
					
					} else { // string
						column[row] = trimVal === '' ? null : trimVal;
						if (row !== 0 && (column.isDatetime || column.isNumeric)) {
							column.mixed = true;
						}
					}
				}
			}

			// If strings are intermixed with numbers or dates in a parsed column, it is an indication
			// that parsing went wrong or the data was not intended to display as numbers or dates and 
			// parsing is too aggressive. Fall back to categories. Demonstrated in the 
			// highcharts/demo/column-drilldown sample.
			if (isXColumn && column.mixed) {
				columns[col] = rawColumns[col];
			}

			// If the 0 column is date or number and descending, reverse all columns. 
			if (isXColumn && descending && this.options.sort) {
				for (col = 0; col < columns.length; col++) {
					columns[col].reverse();
					if (firstRowAsNames) {
						columns[col].unshift(columns[col].pop());
					}
				}
			}
		},
		
		/**
		 * A collection of available date formats, extendable from the outside to support
		 * custom date formats.
		 */
		dateFormats: {
			'YYYY-mm-dd': {
				regex: /^([0-9]{4})[\-\/\.]([0-9]{2})[\-\/\.]([0-9]{2})$/,
				parser: function (match) {
					return Date.UTC(+match[1], match[2] - 1, +match[3]);
				}
			},
			'dd/mm/YYYY': {
				regex: /^([0-9]{1,2})[\-\/\.]([0-9]{1,2})[\-\/\.]([0-9]{4})$/,
				parser: function (match) {
					return Date.UTC(+match[3], match[2] - 1, +match[1]);
				},
				alternative: 'mm/dd/YYYY' // different format with the same regex
			},
			'mm/dd/YYYY': {
				regex: /^([0-9]{1,2})[\-\/\.]([0-9]{1,2})[\-\/\.]([0-9]{4})$/,
				parser: function (match) {
					return Date.UTC(+match[3], match[1] - 1, +match[2]);
				}
			},
			'dd/mm/YY': {
				regex: /^([0-9]{1,2})[\-\/\.]([0-9]{1,2})[\-\/\.]([0-9]{2})$/,
				parser: function (match) {
					return Date.UTC(+match[3] + 2000, match[2] - 1, +match[1]);
				},
				alternative: 'mm/dd/YY' // different format with the same regex
			},
			'mm/dd/YY': {
				regex: /^([0-9]{1,2})[\-\/\.]([0-9]{1,2})[\-\/\.]([0-9]{2})$/,
				parser: function (match) {
					return Date.UTC(+match[3] + 2000, match[1] - 1, +match[2]);
				}
			}
		},
		
		/**
		 * Parse a date and return it as a number. Overridable through options.parseDate.
		 */
		parseDate: function (val) {
			var parseDate = this.options.parseDate,
				ret,
				key,
				format,
				dateFormat = this.options.dateFormat || this.dateFormat,
				match;

			if (parseDate) {
				ret = parseDate(val);
			
			} else if (typeof val === 'string') {
				// Auto-detect the date format the first time
				if (!dateFormat) {
					for (key in this.dateFormats) {
						format = this.dateFormats[key];
						match = val.match(format.regex);
						if (match) {
							this.dateFormat = dateFormat = key;
							this.alternativeFormat = format.alternative;
							ret = format.parser(match);
							break;
						}
					}
				// Next time, use the one previously found
				} else {
					format = this.dateFormats[dateFormat];
					match = val.match(format.regex);
					if (match) {
						ret = format.parser(match);
					}
				}
				// Fall back to Date.parse		
				if (!match) {
					match = Date.parse(val);
					// External tools like Date.js and MooTools extend Date object and
					// returns a date.
					if (typeof match === 'object' && match !== null && match.getTime) {
						ret = match.getTime() - match.getTimezoneOffset() * 60000;
					
					// Timestamp
					} else if (typeof match === 'number' && !isNaN(match)) {
						ret = match - (new Date(match)).getTimezoneOffset() * 60000;
					}
				}
			}
			return ret;
		},
		
		/**
		 * Reorganize rows into columns
		 */
		rowsToColumns: function (rows) {
			var row,
				rowsLength,
				col,
				colsLength,
				columns;

			if (rows) {
				columns = [];
				rowsLength = rows.length;
				for (row = 0; row < rowsLength; row++) {
					colsLength = rows[row].length;
					for (col = 0; col < colsLength; col++) {
						if (!columns[col]) {
							columns[col] = [];
						}
						columns[col][row] = rows[row][col];
					}
				}
			}
			return columns;
		},
		
		/**
		 * A hook for working directly on the parsed columns
		 */
		parsed: function () {
			if (this.options.parsed) {
				return this.options.parsed.call(this, this.columns);
			}
		},

		getFreeIndexes: function (numberOfColumns, seriesBuilders) {
			var s,
				i,
				freeIndexes = [],
				freeIndexValues = [],
				referencedIndexes;

			// Add all columns as free
			for (i = 0; i < numberOfColumns; i = i + 1) {
				freeIndexes.push(true);
			}

			// Loop all defined builders and remove their referenced columns
			for (s = 0; s < seriesBuilders.length; s = s + 1) {
				referencedIndexes = seriesBuilders[s].getReferencedColumnIndexes();

				for (i = 0; i < referencedIndexes.length; i = i + 1) {
					freeIndexes[referencedIndexes[i]] = false;
				}
			}

			// Collect the values for the free indexes
			for (i = 0; i < freeIndexes.length; i = i + 1) {
				if (freeIndexes[i]) {
					freeIndexValues.push(i);
				}
			}

			return freeIndexValues;
		},
		
		/**
		 * If a complete callback function is provided in the options, interpret the 
		 * columns into a Highcharts options object.
		 */
		complete: function () {
			
			var columns = this.columns,
				xColumns = [],
				type,
				options = this.options,
				series,
				data,
				i,
				j,
				r,
				seriesIndex,
				chartOptions,
				allSeriesBuilders = [],
				builder,
				freeIndexes,
				typeCol,
				index;

			xColumns.length = columns.length;
			if (options.complete || options.afterComplete) {

				// Get the names and shift the top row
				for (i = 0; i < columns.length; i++) {
					if (this.firstRowAsNames) {
						columns[i].name = columns[i].shift();
					}
				}
				
				// Use the next columns for series
				series = [];
				freeIndexes = this.getFreeIndexes(columns.length, this.valueCount.seriesBuilders);

				// Populate defined series
				for (seriesIndex = 0; seriesIndex < this.valueCount.seriesBuilders.length; seriesIndex++) {
					builder = this.valueCount.seriesBuilders[seriesIndex];

					// If the builder can be populated with remaining columns, then add it to allBuilders
					if (builder.populateColumns(freeIndexes)) {
						allSeriesBuilders.push(builder);
					}
				}

				// Populate dynamic series
				while (freeIndexes.length > 0) {
					builder = new SeriesBuilder();
					builder.addColumnReader(0, 'x');
					
					// Mark index as used (not free)
					index = inArray(0, freeIndexes);
					if (index !== -1) {
						freeIndexes.splice(index, 1);
					}

					for (i = 0; i < this.valueCount.global; i++) {
						// Create and add a column reader for the next free column index
						builder.addColumnReader(undefined, this.valueCount.globalPointArrayMap[i]);
					}

					// If the builder can be populated with remaining columns, then add it to allBuilders
					if (builder.populateColumns(freeIndexes)) {
						allSeriesBuilders.push(builder);
					}
				}

				// Get the data-type from the first series x column
				if (allSeriesBuilders.length > 0 && allSeriesBuilders[0].readers.length > 0) {
					typeCol = columns[allSeriesBuilders[0].readers[0].columnIndex];
					if (typeCol !== undefined) {
						if (typeCol.isDatetime) {
							type = 'datetime';
						} else if (!typeCol.isNumeric) {
							type = 'category';
						}
					}
				}
				// Axis type is category, then the "x" column should be called "name"
				if (type === 'category') {
					for (seriesIndex = 0; seriesIndex < allSeriesBuilders.length; seriesIndex++) {
						builder = allSeriesBuilders[seriesIndex];
						for (r = 0; r < builder.readers.length; r++) {
							if (builder.readers[r].configName === 'x') {
								builder.readers[r].configName = 'name';
							}
						}
					}
				}

				// Read data for all builders
				for (seriesIndex = 0; seriesIndex < allSeriesBuilders.length; seriesIndex++) {
					builder = allSeriesBuilders[seriesIndex];

					// Iterate down the cells of each column and add data to the series
					data = [];
					for (j = 0; j < columns[0].length; j++) {
						data[j] = builder.read(columns, j);
					}

					// Add the series
					series[seriesIndex] = {
						data: data
					};
					if (builder.name) {
						series[seriesIndex].name = builder.name;
					}
					if (type === 'category') {
						series[seriesIndex].turboThreshold = 0;
					}
				}



				// Do the callback
				chartOptions = {
					series: series
				};
				if (type) {
					chartOptions.xAxis = {
						type: type
					};
				}
				
				if (options.complete) {
					options.complete(chartOptions);
				}

				// The afterComplete hook is used internally to avoid conflict with the externally
				// available complete option.
				if (options.afterComplete) {
					options.afterComplete(chartOptions);
				}
			}
		}
	});
	
	// Register the Data prototype and data function on Highcharts
	Highcharts.Data = Data;
	Highcharts.data = function (options, chartOptions) {
		return new Data(options, chartOptions);
	};

	// Extend Chart.init so that the Chart constructor accepts a new configuration
	// option group, data.
	Highcharts.wrap(Highcharts.Chart.prototype, 'init', function (proceed, userOptions, callback) {
		var chart = this;

		if (userOptions && userOptions.data) {
			Highcharts.data(Highcharts.extend(userOptions.data, {

				afterComplete: function (dataOptions) {
					var i, series;
					
					// Merge series configs
					if (userOptions.hasOwnProperty('series')) {
						if (typeof userOptions.series === 'object') {
							i = Math.max(userOptions.series.length, dataOptions.series.length);
							while (i--) {
								series = userOptions.series[i] || {};
								userOptions.series[i] = Highcharts.merge(series, dataOptions.series[i]);
							}
						} else { // Allow merging in dataOptions.series (#2856)
							delete userOptions.series;
						}
					}

					// Do the merge
					userOptions = Highcharts.merge(dataOptions, userOptions);

					proceed.call(chart, userOptions, callback);
				}
			}), userOptions);
		} else {
			proceed.call(chart, userOptions, callback);
		}
	});

	/**
	 * Creates a new SeriesBuilder. A SeriesBuilder consists of a number
	 * of ColumnReaders that reads columns and give them a name.
	 * Ex: A series builder can be constructed to read column 3 as 'x' and
	 * column 7 and 8 as 'y1' and 'y2'.
	 * The output would then be points/rows of the form {x: 11, y1: 22, y2: 33}
	 * 
	 * The name of the builder is taken from the second column. In the above
	 * example it would be the column with index 7.
	 * @constructor
	 */
	SeriesBuilder = function () {
		this.readers = [];
		this.pointIsArray = true;
	};

	/**
	 * Populates readers with column indexes. A reader can be added without
	 * a specific index and for those readers the index is taken sequentially
	 * from the free columns (this is handled by the ColumnCursor instance).
	 * @returns {boolean}
	 */
	SeriesBuilder.prototype.populateColumns = function (freeIndexes) {
		var builder = this,
			enoughColumns = true;

		// Loop each reader and give it an index if its missing.
		// The freeIndexes.shift() will return undefined if there
		// are no more columns.
		each(builder.readers, function (reader) {
			if (reader.columnIndex === undefined) {
				reader.columnIndex = freeIndexes.shift();
			}
		});

		// Now, all readers should have columns mapped. If not
		// then return false to signal that this series should
		// not be added.
		each(builder.readers, function (reader) {
			if (reader.columnIndex === undefined) {
				enoughColumns = false;
			}
		});

		return enoughColumns;
	};

	/**
	 * Reads a row from the dataset and returns a point or array depending
	 * on the names of the readers.
	 * @param columns
	 * @param rowIndex
	 * @returns {Array | Object}
	 */
	SeriesBuilder.prototype.read = function (columns, rowIndex) {
		var builder = this,
			pointIsArray = builder.pointIsArray,
			point = pointIsArray ? [] : {},
			columnIndexes;

		// Loop each reader and ask it to read its value.
		// Then, build an array or point based on the readers names.
		each(builder.readers, function (reader) {
			var value = columns[reader.columnIndex][rowIndex];
			if (pointIsArray) {
				point.push(value);
			} else {
				point[reader.configName] = value; 
			}
		});

		// The name comes from the first column (excluding the x column)
		if (this.name === undefined && builder.readers.length >= 2) {
			columnIndexes = builder.getReferencedColumnIndexes();
			if (columnIndexes.length >= 2) {
				// remove the first one (x col)
				columnIndexes.shift();

				// Sort the remaining
				columnIndexes.sort();

				// Now use the lowest index as name column
				this.name = columns[columnIndexes.shift()].name;
			}
		}

		return point;
	};

	/**
	 * Creates and adds ColumnReader from the given columnIndex and configName.
	 * ColumnIndex can be undefined and in that case the reader will be given
	 * an index when columns are populated.
	 * @param columnIndex {Number | undefined}
	 * @param configName
	 */
	SeriesBuilder.prototype.addColumnReader = function (columnIndex, configName) {
		this.readers.push({
			columnIndex: columnIndex, 
			configName: configName
		});

		if (!(configName === 'x' || configName === 'y' || configName === undefined)) {
			this.pointIsArray = false;
		}
	};

	/**
	 * Returns an array of column indexes that the builder will use when
	 * reading data.
	 * @returns {Array}
	 */
	SeriesBuilder.prototype.getReferencedColumnIndexes = function () {
		var i,
			referencedColumnIndexes = [],
			columnReader;
		
		for (i = 0; i < this.readers.length; i = i + 1) {
			columnReader = this.readers[i];
			if (columnReader.columnIndex !== undefined) {
				referencedColumnIndexes.push(columnReader.columnIndex);
			}
		}

		return referencedColumnIndexes;
	};

	/**
	 * Returns true if the builder has a reader for the given configName.
	 * @param configName
	 * @returns {boolean}
	 */
	SeriesBuilder.prototype.hasReader = function (configName) {
		var i, columnReader;
		for (i = 0; i < this.readers.length; i = i + 1) {
			columnReader = this.readers[i];
			if (columnReader.configName === configName) {
				return true;
			}
		}
		// Else return undefined
	};



}));
