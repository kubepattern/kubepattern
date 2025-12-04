package it.sigemi.domain.entities.filter;

import lombok.Getter;
import lombok.Setter;
import lombok.ToString;
import lombok.extern.slf4j.Slf4j;

import java.util.List;

@Getter
@Setter
@Slf4j
@ToString
public class ResourceFilter {
    List<FilterRule> matchAll;
    List<FilterRule> matchNone;
    List<FilterRule> matchAny;

    public ResourceFilter(List<FilterRule> matchAll, List<FilterRule> matchAny, List<FilterRule> matchNone) {
        this.matchAll = matchAll;
        this.matchNone = matchNone;
        this.matchAny = matchAny;
        log.debug("ResourceFilter initialized: matchAll={}, matchAny={}, matchNone={}", matchAll, matchAny, matchNone);
    }

    public boolean filterMatchAll(String json) {
        if (matchAll.isEmpty()) {
            log.debug("filterMatchAll: matchAll is empty, returning true");
            return true;
        }
        for (FilterRule filterRule : matchAll) {
            if (!filterRule.match(json)) {
                log.debug("filterMatchAll: rule {} did not match", filterRule);
                return false;
            }
        }
        log.debug("filterMatchAll: all rules matched");
        return true;
    }

    public boolean filterMatchNone(String json) {
        if (matchNone.isEmpty()) {
            log.debug("filterMatchNone: matchNone is empty, returning true");
            return true;
        }
        for (FilterRule filterRule : matchNone) {
            if (filterRule.match(json)) {
                log.debug("filterMatchNone: rule {} matched, returning false", filterRule);
                return false;
            }
        }
        log.debug("filterMatchNone: none of the rules matched");
        return true;
    }

    public boolean filterMatchAny(String json) {
        if (matchAny.isEmpty()) {
            log.debug("filterMatchAny: matchAny is empty, returning true");
            return true;
        }
        for (FilterRule filterRule : matchAny) {
            if (filterRule.match(json)) {
                log.debug("filterMatchAny: rule {} matched", filterRule);
                return true;
            }
        }
        log.debug("filterMatchAny: no rules matched");
        return false;
    }

    public boolean applyFilter(String json) {
        boolean any = filterMatchAny(json);
        boolean none = filterMatchNone(json);
        boolean all = filterMatchAll(json);
        boolean result = any && none && all;
        log.debug("applyFilter: any={}, none={}, all={}, result={}", any, none, all, result);
        return result;
    }
}
