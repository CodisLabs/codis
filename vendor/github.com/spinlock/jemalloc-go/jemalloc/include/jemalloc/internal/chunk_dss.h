/******************************************************************************/
#ifdef JEMALLOC_H_TYPES

typedef enum {
	dss_prec_disabled  = 0,
	dss_prec_primary   = 1,
	dss_prec_secondary = 2,

	dss_prec_limit     = 3
} dss_prec_t;
#define	DSS_PREC_DEFAULT	dss_prec_secondary
#define	DSS_DEFAULT		"secondary"

#endif /* JEMALLOC_H_TYPES */
/******************************************************************************/
#ifdef JEMALLOC_H_STRUCTS

extern const char *dss_prec_names[];

#endif /* JEMALLOC_H_STRUCTS */
/******************************************************************************/
#ifdef JEMALLOC_H_EXTERNS

dss_prec_t	chunk_dss_prec_get(tsdn_t *tsdn);
bool	chunk_dss_prec_set(tsdn_t *tsdn, dss_prec_t dss_prec);
void	*chunk_alloc_dss(tsdn_t *tsdn, arena_t *arena, void *new_addr,
    size_t size, size_t alignment, bool *zero, bool *commit);
bool	chunk_in_dss(tsdn_t *tsdn, void *chunk);
bool	chunk_dss_boot(void);
void	chunk_dss_prefork(tsdn_t *tsdn);
void	chunk_dss_postfork_parent(tsdn_t *tsdn);
void	chunk_dss_postfork_child(tsdn_t *tsdn);

#endif /* JEMALLOC_H_EXTERNS */
/******************************************************************************/
#ifdef JEMALLOC_H_INLINES

#endif /* JEMALLOC_H_INLINES */
/******************************************************************************/
